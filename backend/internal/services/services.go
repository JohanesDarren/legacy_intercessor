// Package services manages Saturday service records and the configurable
// category lookup (PRD 6.3: categories as data, not enum).
package services

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Service is a Saturday 17:00 service record.
type Service struct {
	ID            string     `json:"id"`
	StartsAt      time.Time  `json:"starts_at"`
	Location      string     `json:"location"`
	SlideDeadline *time.Time `json:"slide_deadline"`
	Notes         *string    `json:"notes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Category is a configurable prayer or praise category.
type Category struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository provides service and category persistence.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds the repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// List returns upcoming and recent services, newest first.
func (r *Repository) List(ctx context.Context, includePast bool) ([]Service, error) {
	q := `
		SELECT id, starts_at, location, slide_deadline, notes, created_at, updated_at
		FROM services WHERE deleted_at IS NULL`
	if !includePast {
		q += ` AND starts_at > now() - interval '7 days'`
	}
	q += ` ORDER BY starts_at DESC LIMIT 100`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []Service{}
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.StartsAt, &s.Location, &s.SlideDeadline, &s.Notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get returns one service.
func (r *Repository) Get(ctx context.Context, id string) (*Service, error) {
	var s Service
	err := r.pool.QueryRow(ctx, `
		SELECT id, starts_at, location, slide_deadline, notes, created_at, updated_at
		FROM services WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&s.ID, &s.StartsAt, &s.Location, &s.SlideDeadline, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Service")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

// Next returns the next service starting in the future.
func (r *Repository) Next(ctx context.Context) (*Service, error) {
	var s Service
	err := r.pool.QueryRow(ctx, `
		SELECT id, starts_at, location, slide_deadline, notes, created_at, updated_at
		FROM services WHERE deleted_at IS NULL AND starts_at > now() - interval '3 hours'
		ORDER BY starts_at ASC LIMIT 1
	`).Scan(&s.ID, &s.StartsAt, &s.Location, &s.SlideDeadline, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Upcoming service")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

// Create inserts a service. The slide deadline defaults to Saturday 12:00 WIB
// when not supplied (PRD F-02, Q-08).
func (r *Repository) Create(ctx context.Context, startsAt time.Time, location string, slideDeadline *time.Time, notes *string, actorID string) (*Service, error) {
	if location == "" {
		location = "GBI Aruna, Bandung"
	}
	if slideDeadline == nil {
		d := defaultDeadline(startsAt)
		slideDeadline = &d
	}
	var s Service
	err := r.pool.QueryRow(ctx, `
		INSERT INTO services (starts_at, location, slide_deadline, notes, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING id, starts_at, location, slide_deadline, notes, created_at, updated_at
	`, startsAt, location, slideDeadline, notes, actorID).
		Scan(&s.ID, &s.StartsAt, &s.Location, &s.SlideDeadline, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperr.Conflict("A service already exists at that date and time")
		}
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

// Update patches the mutable fields of a service.
func (r *Repository) Update(ctx context.Context, id string, location *string, slideDeadline *time.Time, notes *string, actorID string) (*Service, error) {
	var s Service
	err := r.pool.QueryRow(ctx, `
		UPDATE services SET
			location       = COALESCE($2, location),
			slide_deadline = COALESCE($3, slide_deadline),
			notes          = COALESCE($4, notes),
			updated_by     = $5,
			updated_at     = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, starts_at, location, slide_deadline, notes, created_at, updated_at
	`, id, location, slideDeadline, notes, actorID).
		Scan(&s.ID, &s.StartsAt, &s.Location, &s.SlideDeadline, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Service")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &s, nil
}

// Categories lists active categories, optionally filtered by kind.
func (r *Repository) Categories(ctx context.Context, kind string) ([]Category, error) {
	q := `SELECT id, kind, name, sort_order, created_at FROM categories WHERE deleted_at IS NULL`
	args := []any{}
	if kind != "" {
		q += ` AND kind = $1`
		args = append(args, kind)
	}
	q += ` ORDER BY kind, sort_order, name`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Kind, &c.Name, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CategoryByID loads one category and validates its kind.
func (r *Repository) CategoryByID(ctx context.Context, id, kind string) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx, `
		SELECT id, kind, name, sort_order, created_at
		FROM categories WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&c.ID, &c.Kind, &c.Name, &c.SortOrder, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Validation("Category does not exist")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if c.Kind != kind {
		return nil, apperr.Validation("Category does not belong to this record kind")
	}
	return &c, nil
}

// defaultDeadline returns 12:00 WIB on the day of the service.
func defaultDeadline(startsAt time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	local := startsAt.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, loc)
}

// Handler exposes service and category endpoints.
type Handler struct{ repo *Repository }

// NewHandler builds the HTTP handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register mounts the routes.
func (h *Handler) Register(g *gin.RouterGroup) {
	all := []string{auth.RoleAdmin, auth.RoleIntercessor, auth.RoleViewer}
	g.GET("/services", auth.RequireRole(all...), h.list)
	g.GET("/services/next", auth.RequireRole(all...), h.next)
	g.GET("/services/:id", auth.RequireRole(all...), h.get)
	g.POST("/services", auth.RequireRole(auth.RoleAdmin), h.create)
	g.PATCH("/services/:id", auth.RequireRole(auth.RoleAdmin), h.update)
	g.GET("/categories", auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor), h.categories)
}

func (h *Handler) list(c *gin.Context) {
	includePast := c.Query("include_past") == "true"
	items, err := h.repo.List(c.Request.Context(), includePast)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Services retrieved", items)
}

func (h *Handler) next(c *gin.Context) {
	s, err := h.repo.Next(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Next service retrieved", s)
}

func (h *Handler) get(c *gin.Context) {
	s, err := h.repo.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Service retrieved", s)
}

type createRequest struct {
	StartsAt      time.Time  `json:"starts_at" binding:"required"`
	Location      string     `json:"location"`
	SlideDeadline *time.Time `json:"slide_deadline"`
	Notes         *string    `json:"notes"`
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("starts_at is required in ISO 8601 format"))
		return
	}
	s, err := h.repo.Create(c.Request.Context(), req.StartsAt, req.Location, req.SlideDeadline, req.Notes, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Service created", s)
}

type updateRequest struct {
	Location      *string    `json:"location"`
	SlideDeadline *time.Time `json:"slide_deadline"`
	Notes         *string    `json:"notes"`
}

func (h *Handler) update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("Invalid request body"))
		return
	}
	s, err := h.repo.Update(c.Request.Context(), c.Param("id"), req.Location, req.SlideDeadline, req.Notes, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Service updated", s)
}

func (h *Handler) categories(c *gin.Context) {
	items, err := h.repo.Categories(c.Request.Context(), c.Query("kind"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Categories retrieved", items)
}
