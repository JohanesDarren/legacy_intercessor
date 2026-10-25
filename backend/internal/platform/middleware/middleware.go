// Package middleware holds the cross-cutting Gin middleware: request ID,
// structured logging, panic recovery, CORS, rate limiting and RBAC.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/ratelimit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Context keys used to pass request-scoped values to handlers and services.
const (
	CtxRequestID = "request_id"
	CtxUserID    = "user_id"
	CtxUserRole  = "user_role"
	CtxUserEmail = "user_email"
)

// RequestID assigns a short unique ID to each request for log correlation.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = "req_" + randHex(6)
		}
		c.Set(CtxRequestID, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// Logger writes one structured JSON line per request. It logs the route
// template rather than the raw path so no record IDs leak into log searches,
// and it never logs bodies, names or tokens.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		elapsed := time.Since(start)

		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		} else if c.Writer.Status() >= 400 {
			level = slog.LevelWarn
		}

		attrs := []any{
			slog.String("request_id", c.GetString(CtxRequestID)),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", elapsed.Milliseconds()),
		}
		if uid := c.GetString(CtxUserID); uid != "" {
			attrs = append(attrs, slog.String("user_id", uid))
		}
		log.Log(c.Request.Context(), level, "http request", attrs...)
	}
}

// Recovery converts a panic into a 500 with the standard envelope.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					slog.String("request_id", c.GetString(CtxRequestID)),
					slog.String("route", c.FullPath()),
					slog.Any("panic", r),
				)
				response.Fail(c, apperr.Internal(nil))
			}
		}()
		c.Next()
	}
}

// CORS allows the configured frontend origins.
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After, X-Request-ID")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RateLimit applies the per-IP fixed-window limiter and sets the documented
// X-RateLimit-* headers on every response.
func RateLimit(limiter *ratelimit.Limiter, limit int, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Next()
			return
		}

		key := c.ClientIP()
		if uid := c.GetString(CtxUserID); uid != "" {
			key = "u:" + uid
		}

		allowed, remaining, resetAt := limiter.Allow(key)
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		if !allowed {
			retry := int(time.Until(resetAt).Seconds())
			if retry < 1 {
				retry = 1
			}
			c.Header("Retry-After", strconv.Itoa(retry))
			response.Fail(c, apperr.RateLimited("Too many requests. Please wait before trying again."))
			return
		}
		c.Next()
	}
}

// BodyLimit rejects request bodies larger than n bytes.
func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		}
		c.Next()
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(b)
}
