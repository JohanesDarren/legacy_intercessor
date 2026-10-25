package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
)

// Role constants used across the app.
const (
	RoleAdmin        = "admin"
	RoleIntercessor  = "intercessor"
	RoleViewer       = "viewer"
	MaxFailedLogins  = 5
	LockDuration     = 15 * time.Minute
	MinPasswordChars = 10
)

// User is the authenticated principal.
type User struct {
	ID           string     `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	IsActive     bool       `json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	PrivacyAckAt *time.Time `json:"privacy_ack_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Service implements authentication, invite handling and token rotation.
type Service struct {
	pool      *pgxpool.Pool
	tokens    *TokenIssuer
	refreshTTL time.Duration
	bcryptCost int
	inviteTTL  time.Duration
	log       *slog.Logger
}

// NewService wires the auth service.
func NewService(pool *pgxpool.Pool, tokens *TokenIssuer, refreshTTL time.Duration, bcryptCost int, inviteTTL time.Duration, log *slog.Logger) *Service {
	return &Service{
		pool:       pool,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		bcryptCost: bcryptCost,
		inviteTTL:  inviteTTL,
		log:        log,
	}
}

// TokenPair is returned by login and refresh.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         *User     `json:"user"`
}

// Login verifies credentials, enforces lockout and issues a token pair.
func (s *Service) Login(ctx context.Context, email, password, ip string) (*TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var (
		id           string
		fullName     string
		role         string
		hash         *string
		isActive     bool
		failedLogins int
		lockedUntil  *time.Time
		privacyAck   *time.Time
		lastLogin    *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, email, role, password_hash, is_active,
		       failed_logins, locked_until, privacy_ack_at, last_login_at
		FROM users WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(&id, &fullName, &email, &role, &hash, &isActive, &failedLogins, &lockedUntil, &privacyAck, &lastLogin)

	if errors.Is(err, pgx.ErrNoRows) {
		s.log.Info("login failed", slog.String("result", "unknown_email"), slog.String("ip", ip))
		return nil, apperr.InvalidCredentials()
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}

	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		s.log.Info("login blocked", slog.String("user_id", id), slog.String("result", "locked"))
		return nil, apperr.Locked("Account locked after repeated failed logins. Try again later.")
	}
	if !isActive {
		return nil, apperr.Forbidden("This account is inactive. Contact the lead intercessor.")
	}
	if hash == nil {
		return nil, apperr.Forbidden("This account has no password yet. Use the invite link to set one.")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password)); err != nil {
		s.registerFailure(ctx, id, failedLogins)
		s.log.Info("login failed", slog.String("user_id", id), slog.String("result", "bad_password"), slog.String("ip", ip))
		return nil, apperr.InvalidCredentials()
	}

	// Success: reset counters, stamp last_login.
	now := time.Now()
	if _, err := s.pool.Exec(ctx, `
		UPDATE users SET failed_logins = 0, locked_until = NULL,
		       last_login_at = $2, updated_at = now()
		WHERE id = $1
	`, id, now); err != nil {
		return nil, apperr.Internal(err)
	}

	pair, err := s.issuePair(ctx, &User{
		ID: id, FullName: fullName, Email: email, Role: role,
		IsActive: isActive, LastLoginAt: &now, PrivacyAckAt: privacyAck,
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("login success", slog.String("user_id", id), slog.String("role", role), slog.String("ip", ip))
	return pair, nil
}

// registerFailure increments the failure counter and locks the account at the
// configured threshold.
func (s *Service) registerFailure(ctx context.Context, userID string, current int) {
	next := current + 1
	if next >= MaxFailedLogins {
		until := time.Now().Add(LockDuration)
		_, err := s.pool.Exec(ctx, `
			UPDATE users SET failed_logins = $2, locked_until = $3, updated_at = now()
			WHERE id = $1
		`, userID, next, until)
		if err == nil {
			s.log.Warn("account locked", slog.String("user_id", userID), slog.Time("locked_until", until))
		}
		return
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE users SET failed_logins = $2, updated_at = now() WHERE id = $1
	`, userID, next)
}

// issuePair creates an access token and a rotating refresh token.
func (s *Service) issuePair(ctx context.Context, u *User) (*TokenPair, error) {
	access, err := s.tokens.Issue(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	refresh := randToken(32)
	expiresAt := time.Now().Add(s.refreshTTL)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, u.ID, hashToken(refresh), expiresAt); err != nil {
		return nil, apperr.Internal(err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.tokens.TTL().Seconds()),
		ExpiresAt:    time.Now().Add(s.tokens.TTL()),
		User:         u,
	}, nil
}

// Refresh rotates a refresh token: the presented token is revoked and a new
// pair is issued. Reuse of a revoked token is rejected.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, apperr.Unauthorized("Refresh token is required")
	}
	digest := hashToken(refreshToken)

	var (
		tokenID  string
		userID   string
		expires  time.Time
		revoked  *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1
	`, digest).Scan(&tokenID, &userID, &expires, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Unauthorized("Refresh token is invalid")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if revoked != nil {
		// A revoked token being reused suggests theft: revoke the whole family.
		_, _ = s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		s.log.Warn("refresh token reuse detected", slog.String("user_id", userID))
		return nil, apperr.Unauthorized("Refresh token has already been used")
	}
	if time.Now().After(expires) {
		return nil, apperr.Unauthorized("Refresh token has expired")
	}

	var u User
	err = s.pool.QueryRow(ctx, `
		SELECT id, full_name, email, role, is_active, last_login_at, privacy_ack_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.IsActive, &u.LastLoginAt, &u.PrivacyAckAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Unauthorized("Account no longer exists")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !u.IsActive {
		return nil, apperr.Forbidden("This account is inactive")
	}

	if _, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, tokenID); err != nil {
		return nil, apperr.Internal(err)
	}
	return s.issuePair(ctx, &u)
}

// Logout revokes the presented refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hashToken(refreshToken))
	if err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// Authenticate resolves the user behind an access token and rejects stale
// roles by re-reading the row.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (*User, error) {
	claims, err := s.tokens.Verify(accessToken)
	if err != nil {
		return nil, apperr.Unauthorized("Access token is invalid or expired")
	}
	var u User
	err = s.pool.QueryRow(ctx, `
		SELECT id, full_name, email, role, is_active, last_login_at, privacy_ack_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, claims.UserID).Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.IsActive, &u.LastLoginAt, &u.PrivacyAckAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Unauthorized("Account no longer exists")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !u.IsActive {
		return nil, apperr.Forbidden("This account is inactive")
	}
	return &u, nil
}

// HashPassword validates length and returns a bcrypt hash at the configured cost.
func (s *Service) HashPassword(password string) (string, error) {
	if len([]rune(password)) < MinPasswordChars {
		return "", apperr.Validation("Password must be at least 10 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", apperr.Internal(err)
	}
	return string(b), nil
}
