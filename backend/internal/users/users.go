// Package users handles profiles, the user list, and the UU PDP access and
// deletion rights from PRD Section 10.2.
package users

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Service provides user reads and self-service updates.
type Service struct {
	pool *pgxpool.Pool
}

// NewService builds the users service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Profile is the detailed user record returned to the owner or an admin.
type Profile struct {
	auth.User
	UpcomingShifts int `json:"upcoming_shifts"`
}

// Get returns one user by ID.
func (s *Service) Get(ctx context.Context, id string) (*auth.User, error) {
	var u auth.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, email, role, is_active, last_login_at, privacy_ack_at, created_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.IsActive, &u.LastLoginAt, &u.PrivacyAckAt, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("User")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &u, nil
}

// List returns all active users, ordered by name.
func (s *Service) List(ctx context.Context) ([]auth.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, full_name, email, role, is_active, last_login_at, privacy_ack_at, created_at
		FROM users WHERE deleted_at IS NULL
		ORDER BY is_active DESC, full_name ASC
	`)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []auth.User{}
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.IsActive, &u.LastLoginAt, &u.PrivacyAckAt, &u.CreatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateSelf changes the caller's own name, and optionally password.
func (s *Service) UpdateSelf(ctx context.Context, userID, fullName, currentPassword, newPassword string) (*auth.User, error) {
	if fullName != "" {
		if _, err := s.pool.Exec(ctx, `
			UPDATE users SET full_name = $2, updated_at = now() WHERE id = $1
		`, userID, fullName); err != nil {
			return nil, apperr.Internal(err)
		}
	}

	if newPassword != "" {
		if len([]rune(newPassword)) < auth.MinPasswordChars {
			return nil, apperr.Validation("Password must be at least 10 characters")
		}
		var hash *string
		if err := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
			return nil, apperr.Internal(err)
		}
		if hash != nil {
			if err := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(currentPassword)); err != nil {
				return nil, apperr.Validation("Current password is incorrect")
			}
		}
		newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		if _, err := s.pool.Exec(ctx, `
			UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1
		`, userID, string(newHash)); err != nil {
			return nil, apperr.Internal(err)
		}
		// Password change invalidates existing refresh tokens.
		if _, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	return s.Get(ctx, userID)
}

// AcknowledgePrivacy stamps the internal data notice acknowledgement.
func (s *Service) AcknowledgePrivacy(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET privacy_ack_at = COALESCE(privacy_ack_at, now()), updated_at = now()
		WHERE id = $1
	`, userID)
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// SetActive enables or disables an account (admin action).
func (s *Service) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL
	`, id, active)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("User")
	}
	if !active {
		_, _ = s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id)
	}
	return nil
}

// Export builds the caller's own data bundle (UU PDP right of access and
// portability). It returns everything the user is the author of.
func (s *Service) Export(ctx context.Context, userID string) (map[string]any, error) {
	profile, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"exported_at":     time.Now().UTC(),
		"profile":         profile,
		"prayer_requests": []any{},
		"praise_reports":  []any{},
		"follow_ups":      []any{},
		"booth_shifts":    []any{},
	}

	prs, err := collectRows(ctx, s.pool, `
		SELECT id, title, content, is_urgent, status, display_consent, received_at, created_at
		FROM prayer_requests WHERE created_by = $1 AND deleted_at IS NULL ORDER BY received_at DESC
	`, userID, []string{"id", "title", "content", "is_urgent", "status", "display_consent", "received_at", "created_at"})
	if err != nil {
		return nil, err
	}
	out["prayer_requests"] = prs

	praise, err := collectRows(ctx, s.pool, `
		SELECT id, title, content, is_featured, display_consent, reported_at, created_at
		FROM praise_reports WHERE created_by = $1 AND deleted_at IS NULL ORDER BY reported_at DESC
	`, userID, []string{"id", "title", "content", "is_featured", "display_consent", "reported_at", "created_at"})
	if err != nil {
		return nil, err
	}
	out["praise_reports"] = praise

	fus, err := collectRows(ctx, s.pool, `
		SELECT id, prayer_request_id, contact_date, channel, update_type, condition_status, notes, next_action, next_action_date
		FROM follow_ups WHERE created_by = $1 AND deleted_at IS NULL ORDER BY contact_date DESC
	`, userID, []string{"id", "prayer_request_id", "contact_date", "channel", "update_type", "condition_status", "notes", "next_action", "next_action_date"})
	if err != nil {
		return nil, err
	}
	out["follow_ups"] = fus

	shifts, err := collectRows(ctx, s.pool, `
		SELECT ba.id, bs.label, bs.starts_at, bs.ends_at
		FROM booth_assignments ba
		JOIN booth_shifts bs ON bs.id = ba.shift_id
		WHERE ba.user_id = $1 AND ba.deleted_at IS NULL AND bs.deleted_at IS NULL
		ORDER BY bs.starts_at DESC
	`, userID, []string{"id", "label", "starts_at", "ends_at"})
	if err != nil {
		return nil, err
	}
	out["booth_shifts"] = shifts

	return out, nil
}

// DeleteSelf anonymises the account: personal fields are cleared and the
// account is deactivated. Records stay for church record-keeping but no longer
// identify the user.
func (s *Service) DeleteSelf(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	anonEmail := "deleted+" + userID[:8] + "@legacy.invalid"
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET full_name = 'Deleted user', email = $2, password_hash = NULL,
		    is_active = false, deleted_at = now(), updated_at = now()
		WHERE id = $1
	`, userID, anonEmail); err != nil {
		return apperr.Internal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return apperr.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// collectRows runs a query and returns rows as generic maps with typed values.
func collectRows(ctx context.Context, pool *pgxpool.Pool, sql, arg string, cols []string) ([]map[string]any, error) {
	rows, err := pool.Query(ctx, sql, arg)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, apperr.Internal(err)
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = values[i]
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Handler exposes user endpoints.
type Handler struct{ svc *Service }

// NewHandler builds the users HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register mounts user routes.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/users/me", h.me)
	g.PATCH("/users/me", h.updateMe)
	g.POST("/users/me/privacy-ack", h.ackPrivacy)
	g.GET("/users/me/export", h.exportMe)
	g.DELETE("/users/me", h.deleteMe)
	g.GET("/users", auth.RequireRole(auth.RoleAdmin), h.list)
	g.PATCH("/users/:id/active", auth.RequireRole(auth.RoleAdmin), h.setActive)
}

func (h *Handler) me(c *gin.Context) {
	u, err := h.svc.Get(c.Request.Context(), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Profile retrieved", u)
}

type updateMeRequest struct {
	FullName        string `json:"full_name"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) updateMe(c *gin.Context) {
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("Invalid request body"))
		return
	}
	u, err := h.svc.UpdateSelf(c.Request.Context(), c.GetString(middleware.CtxUserID), req.FullName, req.CurrentPassword, req.NewPassword)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Profile updated", u)
}

func (h *Handler) ackPrivacy(c *gin.Context) {
	if err := h.svc.AcknowledgePrivacy(c.Request.Context(), c.GetString(middleware.CtxUserID)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Privacy notice acknowledged", gin.H{"acknowledged": true})
}

func (h *Handler) exportMe(c *gin.Context) {
	data, err := h.svc.Export(c.Request.Context(), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="legacy-intercessor-my-data.json"`)
	response.OK(c, "Personal data exported", data)
}

func (h *Handler) deleteMe(c *gin.Context) {
	if err := h.svc.DeleteSelf(c.Request.Context(), c.GetString(middleware.CtxUserID)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Account deleted and anonymised", gin.H{"deleted": true})
}

func (h *Handler) list(c *gin.Context) {
	users, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Users retrieved", users)
}

type setActiveRequest struct {
	IsActive bool `json:"is_active"`
}

func (h *Handler) setActive(c *gin.Context) {
	var req setActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("is_active is required"))
		return
	}
	if err := h.svc.SetActive(c.Request.Context(), c.Param("id"), req.IsActive); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "User updated", gin.H{"id": c.Param("id"), "is_active": req.IsActive})
}
