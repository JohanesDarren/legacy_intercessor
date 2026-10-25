package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// RequireAuth validates the bearer token and stores the principal in context.
func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			response.Fail(c, apperr.Unauthorized("Authorization header with a bearer token is required"))
			return
		}
		raw := strings.TrimSpace(header[7:])
		u, err := svc.Authenticate(c.Request.Context(), raw)
		if err != nil {
			response.Fail(c, err)
			return
		}
		c.Set(middleware.CtxUserID, u.ID)
		c.Set(middleware.CtxUserRole, u.Role)
		c.Set(middleware.CtxUserEmail, u.Email)
		c.Next()
	}
}

// RequireRole allows only the listed roles. Every protected endpoint must
// declare its roles explicitly (PRD 4.4, RBAC on every endpoint).
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role := c.GetString(middleware.CtxUserRole)
		if !allowed[role] {
			response.Fail(c, apperr.Forbidden("Your role does not have access to this resource"))
			return
		}
		c.Next()
	}
}

// Invite is a pending invitation.
type Invite struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	FullName  string     `json:"full_name"`
	Role      string     `json:"role"`
	ExpiresAt time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	CreatedAt time.Time  `json:"created_at"`
	// Token is only populated in the create response so the admin can copy
	// the invite link. It is never persisted in plain text.
	Token string `json:"invite_token,omitempty"`
}

// CreateInvite issues a 72-hour invitation. The raw token is returned once.
func (s *Service) CreateInvite(ctx context.Context, actorID, email, fullName, role string) (*Invite, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	fullName = strings.TrimSpace(fullName)

	fields := map[string][]string{}
	if !strings.Contains(email, "@") || len(email) < 5 {
		fields["email"] = []string{"A valid email address is required"}
	}
	if role != RoleAdmin && role != RoleIntercessor && role != RoleViewer {
		fields["role"] = []string{"Role must be admin, intercessor or viewer"}
	}
	if fullName == "" {
		fields["full_name"] = []string{"Full name is required"}
	}
	if len(fields) > 0 {
		return nil, apperr.ValidationFields("Please correct the highlighted fields", fields)
	}

	var existing string
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`, email).Scan(&existing)
	if err == nil {
		return nil, apperr.Conflict("A user with this email already exists")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Internal(err)
	}

	token := randToken(32)
	expires := time.Now().Add(s.inviteTTL)

	var inv Invite
	err = s.pool.QueryRow(ctx, `
		INSERT INTO user_invites (email, full_name, role, token_hash, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, full_name, role, expires_at, accepted_at, created_at
	`, email, fullName, role, hashToken(token), expires, actorID).
		Scan(&inv.ID, &inv.Email, &inv.FullName, &inv.Role, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	inv.Token = token

	s.log.Info("invite sent", slog.String("role", role), slog.String("result", "created"))
	return &inv, nil
}

// InvitePreview describes an invite for the accept screen.
type InvitePreview struct {
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PreviewInvite validates a token and returns what the accept screen shows.
func (s *Service) PreviewInvite(ctx context.Context, token string) (*InvitePreview, error) {
	var (
		p          InvitePreview
		acceptedAt *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT email, COALESCE(full_name, ''), role, expires_at, accepted_at
		FROM user_invites WHERE token_hash = $1
	`, hashToken(token)).Scan(&p.Email, &p.FullName, &p.Role, &p.ExpiresAt, &acceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Validation("This invite link is not valid")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if acceptedAt != nil {
		return nil, apperr.Unprocessable(apperr.CodeInviteUsed, "This invite has already been used")
	}
	if time.Now().After(p.ExpiresAt) {
		return nil, apperr.Unprocessable(apperr.CodeInviteExpired, "This invite has expired. Ask the admin for a new one.")
	}
	return &p, nil
}

// AcceptInvite sets the password and creates the account.
func (s *Service) AcceptInvite(ctx context.Context, token, password, fullName string) (*TokenPair, error) {
	preview, err := s.PreviewInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	hash, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fullName) == "" {
		fullName = preview.FullName
	}
	if strings.TrimSpace(fullName) == "" {
		fullName = strings.Split(preview.Email, "@")[0]
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (full_name, email, password_hash, role, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, full_name, email, role, is_active, last_login_at, privacy_ack_at, created_at
	`, fullName, preview.Email, hash, preview.Role).
		Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.IsActive, &u.LastLoginAt, &u.PrivacyAckAt, &u.CreatedAt)
	if err != nil {
		return nil, apperr.Conflict("An account with this email already exists")
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_invites SET accepted_at = now() WHERE token_hash = $1
	`, hashToken(token)); err != nil {
		return nil, apperr.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}

	s.log.Info("invite accepted", slog.String("user_id", u.ID), slog.String("role", u.Role), slog.String("result", "accepted"))
	return s.issuePair(ctx, &u)
}
