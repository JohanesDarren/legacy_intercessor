// Command api runs the Legacy Intercessor REST API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/ai"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/audit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/booth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/config"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/dashboard"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/followup"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/db"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/logger"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/ratelimit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/storage"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/praise"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/prayer"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/presentation"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/services"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/users"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Info("database connected", slog.String("env", cfg.Env))

	if err := applyMigrations(ctx, pool, log); err != nil {
		return err
	}
	if err := seedUsers(ctx, pool, cfg, log); err != nil {
		return err
	}

	// ---- platform -------------------------------------------------------
	auditLog := audit.New(pool, log)
	store, err := storage.NewLocal(cfg.LocalStorageDir, cfg.ExportBaseURL)
	if err != nil {
		return err
	}
	limiter := ratelimit.New(cfg.RateLimitPerMinute, time.Minute)

	// ---- repositories ---------------------------------------------------
	serviceRepo := services.NewRepository(pool)
	prayerRepo := prayer.NewRepository(pool, auditLog)
	praiseRepo := praise.NewRepository(pool, auditLog)
	followRepo := followup.NewRepository(pool, auditLog)
	boothRepo := booth.NewRepository(pool, auditLog)

	// ---- services -------------------------------------------------------
	tokenIssuer := auth.NewTokenIssuer(cfg.JWTAccessSecret, cfg.JWTAccessTTL)
	authSvc := auth.NewService(pool, tokenIssuer, cfg.JWTRefreshTTL, cfg.BcryptCost, cfg.InviteTTL, log)
	userSvc := users.NewService(pool)

	provider := ai.NewProvider(cfg.AIProvider, cfg.AIAPIKey, cfg.AIModel, cfg.AITimeout)
	aiSvc := ai.NewService(pool, provider, auditLog, log, cfg.AITimeout)
	log.Info("ai provider ready",
		slog.String("provider", provider.Name()),
		slog.String("model", provider.Model()),
		slog.Bool("offline_mode", provider.Name() == "offline"),
	)

	renderer, err := presentation.NewRenderer(presentation.RendererConfig{
		TemplateDir: cfg.TemplateDir,
		ChromeBin:   cfg.ChromeBin,
		Timeout:     cfg.ExportTimeout,
	})
	if err != nil {
		return err
	}

	builder := &presentation.Builder{
		RecordsFor: func(ctx context.Context, serviceID, kind string) ([]presentation.RecordSource, error) {
			return loadRecords(ctx, serviceID, kind, prayerRepo, praiseRepo)
		},
		ApprovedSummaryFor: func(ctx context.Context, serviceID, kind string) (map[string]string, error) {
			approved, err := aiSvc.ApprovedFor(ctx, serviceID, kind)
			if err != nil {
				return nil, err
			}
			out := make(map[string]string, len(approved))
			for catID, sm := range approved {
				out[catID] = sm.DisplayText
			}
			return out, nil
		},
		DraftCount: func(ctx context.Context, serviceID, kind string) (int, error) {
			return aiSvc.UnapprovedCount(ctx, serviceID, kind)
		},
	}
	presSvc := presentation.NewService(pool, renderer, store, builder, auditLog, log,
		cfg.ExportBaseURL, cfg.MaxExportConcurrency)

	dashSvc := dashboard.NewService(pool, serviceRepo, boothRepo)

	// ---- HTTP -----------------------------------------------------------
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.Logger(log),
		middleware.CORS(cfg.CORS),
		middleware.BodyLimit(2<<20),
	)
	router.RedirectTrailingSlash = false

	v1 := router.Group("/api/v1")
	v1.Use(middleware.RateLimit(limiter, cfg.RateLimitPerMinute, !cfg.DisableRateLimit))

	// Health and version endpoints stay public for uptime monitoring (PRD 6.6).
	v1.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, "Healthy", gin.H{"status": "ok", "time": time.Now()})
	})
	v1.GET("/meta/version", func(c *gin.Context) {
		response.OK(c, "Version info", gin.H{
			"api_version":           "v1",
			"min_app_version":       "1.0.0",
			"ai_provider":           provider.Name(),
			"offline_mode":          provider.Name() == "offline",
			"environment":           cfg.Env,
			"export_driver":         store.Driver(),
			"timezone":              cfg.Timezone,
			"rate_limit_per_minute": cfg.RateLimitPerMinute,
		})
	})

	// Public auth routes (login, refresh, accept-invite, invite preview).
	authHandler := auth.NewHandler(authSvc)
	authHandler.RegisterPublic(v1)

	// Protected routes: every endpoint declares its roles.
	protected := v1.Group("")
	protected.Use(auth.RequireAuth(authSvc))

	authHandler.RegisterProtected(protected)
	users.NewHandler(userSvc).Register(protected)
	services.NewHandler(serviceRepo).Register(protected)
	prayer.NewHandler(prayerRepo).Register(protected)
	praise.NewHandler(praiseRepo).Register(protected)
	followup.NewHandler(followRepo).Register(protected)
	booth.NewHandler(boothRepo).Register(protected)
	dashboard.NewHandler(dashSvc).Register(protected)
	presentation.NewHandler(presSvc).Register(protected)
	presentation.NewFileHandler(cfg.LocalStorageDir).Register(protected)

	aiHandler := ai.NewHandler(aiSvc)
	aiHandler.Register(protected, func(c *gin.Context, serviceID, kind string) ([]ai.SourceRecord, error) {
		records, err := loadRecords(c.Request.Context(), serviceID, kind, prayerRepo, praiseRepo)
		if err != nil {
			return nil, err
		}
		out := make([]ai.SourceRecord, 0, len(records))
		for _, r := range records {
			out = append(out, ai.SourceRecord{
				ID: r.ID, CategoryID: r.CategoryID, CategoryName: r.CategoryName,
				Title: r.Title, Content: r.Content, Name: r.Name,
				DisplayConsent: r.DisplayConsent,
			})
		}
		return out, nil
	})

	// Serve the built PWA when configured (single-container deployment).
	if cfg.FrontendDistDir != "" {
		router.Static("/assets", cfg.FrontendDistDir+"/assets")
		router.StaticFile("/", cfg.FrontendDistDir+"/index.html")
		router.NoRoute(func(c *gin.Context) {
			c.File(cfg.FrontendDistDir + "/index.html")
		})
	}

	router.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("api listening", slog.String("port", cfg.Port), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", slog.Any("error", err))
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("shutdown complete")
	return nil
}

// loadRecords returns the eligible records for a service and kind in the
// narrow shape the AI and presentation layers need.
func loadRecords(ctx context.Context, serviceID, kind string, prayerRepo *prayer.Repository, praiseRepo *praise.Repository) ([]presentation.RecordSource, error) {
	out := []presentation.RecordSource{}

	if kind == "praise" {
		items, err := praiseRepo.Eligible(ctx, serviceID)
		if err != nil {
			return nil, err
		}
		for _, r := range items {
			name := ""
			if r.ReporterName != nil {
				name = *r.ReporterName
			}
			out = append(out, presentation.RecordSource{
				ID: r.ID, CategoryID: r.CategoryID, CategoryName: r.CategoryName,
				Title: r.Title, Content: r.Content, Name: name,
				DisplayConsent: r.DisplayConsent, IsFeatured: r.IsFeatured,
			})
		}
		return out, nil
	}

	items, err := prayerRepo.Eligible(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	for _, r := range items {
		name := ""
		if r.RequesterName != nil {
			name = *r.RequesterName
		}
		out = append(out, presentation.RecordSource{
			ID: r.ID, CategoryID: r.CategoryID, CategoryName: r.CategoryName,
			Title: r.Title, Content: r.Content, Name: name,
			DisplayConsent: r.DisplayConsent, IsUrgent: r.IsUrgent,
		})
	}
	return out, nil
}
