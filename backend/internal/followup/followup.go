// Package followup implements the urgent follow-up log (PRD F-08). These notes
// are internal only: they never reach slides or AI prompts.
package followup

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/audit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/conv"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Entry is one logged contact with the person behind a request.
type Entry struct {
	ID              string     `json:"id"`
	PrayerRequestID string     `json:"prayer_request_id"`
	ContactDate     string     `json:"contact_date"`
	Channel         string     `json:"channel"`
	UpdateType      string     `json:"update_type"`
	ConditionStatus string     `json:"condition_status"`
	Notes           string     `json:"notes"`
	NextAction      *string    `json:"next_action"`
	NextActionDate  *string    `json:"next_action_date"`
	CreatedAt       time.Time  `json:"created_at"`
	CreatedBy       *string    `json:"created_by"`
	CreatedByName   *string    `json:"created_by_name"`
	// RequestTitle is included so the timeline is readable without a second call.
	RequestTitle string `json:"request_title,omitempty"`
	IsUrgent     bool   `json:"is_urgent,omitempty"`
}

// Repository persists follow-ups.
type Repository struct {
	pool  *pgxpool.Pool
	audit *audit.Logger
}

// NewRepository builds the repository.
func NewRepository(pool *pgxpool.Pool, auditLog *audit.Logger) *Repository {
	return &Repository{pool: pool, audit: auditLog}
}

const selectColumns = `
	f.id, f.prayer_request_id, to_char(f.contact_date, 'YYYY-MM-DD'),
	f.channel, f.update_type, f.condition_status, f.notes,
	f.next_action, to_char(f.next_action_date, 'YYYY-MM-DD'),
	f.created_at, f.created_by, u.full_name
`

// ListByRequest returns the timeline for one request, newest first.
func (r *Repository) ListByRequest(ctx context.Context, prayerRequestID string) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+selectColumns+`
		FROM follow_ups f
		LEFT JOIN users u ON u.id = f.created_by
		WHERE f.prayer_request_id = $1 AND f.deleted_at IS NULL
		ORDER BY f.contact_date DESC, f.created_at DESC
	`, prayerRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// ListRecent returns the latest follow-ups across all requests.
func (r *Repository) ListRecent(ctx context.Context, limit int) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+selectColumns+`, p.title, p.is_urgent
		FROM follow_ups f
		LEFT JOIN users u ON u.id = f.created_by
		JOIN prayer_requests p ON p.id = f.prayer_request_id
		WHERE f.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY f.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.PrayerRequestID, &e.ContactDate, &e.Channel, &e.UpdateType,
			&e.ConditionStatus, &e.Notes, &e.NextAction, &e.NextActionDate,
			&e.CreatedAt, &e.CreatedBy, &e.CreatedByName, &e.RequestTitle, &e.IsUrgent); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanEntries(rows pgx.Rows) ([]Entry, error) {
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.PrayerRequestID, &e.ContactDate, &e.Channel, &e.UpdateType,
			&e.ConditionStatus, &e.Notes, &e.NextAction, &e.NextActionDate,
			&e.CreatedAt, &e.CreatedBy, &e.CreatedByName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateInput carries a validated follow-up payload.
type CreateInput struct {
	ContactDate     time.Time
	Channel         string
	UpdateType      string
	ConditionStatus string
	Notes           string
	NextAction      *string
	NextActionDate  *time.Time
}

// Create logs a follow-up against a prayer request.
func (r *Repository) Create(ctx context.Context, prayerRequestID string, in CreateInput, actorID, ip string) (*Entry, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM prayer_requests WHERE id = $1 AND deleted_at IS NULL)`, prayerRequestID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRequestNotFound
	}

	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO follow_ups
			(prayer_request_id, contact_date, channel, update_type, condition_status,
			 notes, next_action, next_action_date, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id
	`, prayerRequestID, in.ContactDate, in.Channel, in.UpdateType, in.ConditionStatus,
		in.Notes, in.NextAction, in.NextActionDate, actorID).Scan(&id)
	if err != nil {
		return nil, err
	}

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "follow_up.create", Entity: "follow_ups",
		EntityID: id, IP: ip,
	})
	return r.Get(ctx, id)
}

// Get returns one follow-up.
func (r *Repository) Get(ctx context.Context, id string) (*Entry, error) {
	var e Entry
	err := r.pool.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM follow_ups f
		LEFT JOIN users u ON u.id = f.created_by
		WHERE f.id = $1 AND f.deleted_at IS NULL
	`, id).Scan(&e.ID, &e.PrayerRequestID, &e.ContactDate, &e.Channel, &e.UpdateType,
		&e.ConditionStatus, &e.Notes, &e.NextAction, &e.NextActionDate,
		&e.CreatedAt, &e.CreatedBy, &e.CreatedByName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// UpdateInput carries a correction payload.
type UpdateInput struct {
	Channel         *string
	UpdateType      *string
	ConditionStatus *string
	Notes           *string
	NextAction      *string
	NextActionDate  *time.Time
	ContactDate     *time.Time
}

// Update corrects a follow-up entry.
func (r *Repository) Update(ctx context.Context, id string, in UpdateInput, actorID, ip string) (*Entry, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE follow_ups SET
			channel          = COALESCE($2, channel),
			update_type      = COALESCE($3, update_type),
			condition_status = COALESCE($4, condition_status),
			notes            = COALESCE($5, notes),
			next_action      = COALESCE($6, next_action),
			next_action_date = COALESCE($7, next_action_date),
			contact_date     = COALESCE($8, contact_date),
			updated_by       = $9,
			updated_at       = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, in.Channel, in.UpdateType, in.ConditionStatus, in.Notes,
		in.NextAction, in.NextActionDate, in.ContactDate, actorID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "follow_up.update", Entity: "follow_ups", EntityID: id, IP: ip,
	})
	return r.Get(ctx, id)
}

// Owner returns the creator of a follow-up, used to enforce "owner or admin".
func (r *Repository) Owner(ctx context.Context, id string) (string, error) {
	var owner *string
	err := r.pool.QueryRow(ctx, `SELECT created_by FROM follow_ups WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if owner == nil {
		return "", nil
	}
	return *owner, nil
}

// Errors returned by the repository.
var (
	ErrNotFound        = errors.New("follow-up not found")
	ErrRequestNotFound = errors.New("prayer request not found")
)

// Handler exposes follow-up endpoints.
type Handler struct{ repo *Repository }

// NewHandler builds the HTTP handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register mounts follow-up routes.
func (h *Handler) Register(g *gin.RouterGroup) {
	writers := auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor)
	g.GET("/prayer-requests/:id/follow-ups", writers, h.list)
	g.POST("/prayer-requests/:id/follow-ups", writers, h.create)
	g.GET("/follow-ups", writers, h.recent)
	g.PATCH("/follow-ups/:id", writers, h.update)
}

func (h *Handler) list(c *gin.Context) {
	items, err := h.repo.ListByRequest(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.OK(c, "Follow-ups retrieved", items)
}

func (h *Handler) recent(c *gin.Context) {
	limit := conv.Int(c.Query("limit"), 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}
	items, err := h.repo.ListRecent(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.OK(c, "Recent follow-ups retrieved", items)
}

type createPayload struct {
	ContactDate     string  `json:"contact_date"`
	Channel         string  `json:"channel" binding:"required"`
	UpdateType      string  `json:"update_type" binding:"required"`
	ConditionStatus string  `json:"condition_status" binding:"required"`
	Notes           string  `json:"notes" binding:"required"`
	NextAction      *string `json:"next_action"`
	NextActionDate  *string `json:"next_action_date"`
}

func (h *Handler) create(c *gin.Context) {
	var p createPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("channel, update_type, condition_status and notes are required"))
		return
	}

	fields := map[string][]string{}
	if !validChannel(p.Channel) {
		fields["channel"] = []string{"Channel must be visit, phone, whatsapp or other"}
	}
	if !validUpdateType(p.UpdateType) {
		fields["update_type"] = []string{"Update type must be documentation, life_update, current_condition or other"}
	}
	if !validCondition(p.ConditionStatus) {
		fields["condition_status"] = []string{"Condition must be improving, stable, worsening, answered or unreachable"}
	}
	if strings.TrimSpace(p.Notes) == "" {
		fields["notes"] = []string{"Notes are required"}
	} else if len(p.Notes) > 5000 {
		fields["notes"] = []string{"Notes must be at most 5000 characters"}
	}
	if len(fields) > 0 {
		response.Fail(c, apperr.ValidationFields("Please correct the highlighted fields", fields))
		return
	}

	contactDate := time.Now()
	if p.ContactDate != "" {
		d, err := conv.ParseDate(p.ContactDate)
		if err != nil {
			response.Fail(c, apperr.Validation("contact_date must be YYYY-MM-DD"))
			return
		}
		contactDate = d
	}

	in := CreateInput{
		ContactDate:     contactDate,
		Channel:         p.Channel,
		UpdateType:      p.UpdateType,
		ConditionStatus: p.ConditionStatus,
		Notes:           strings.TrimSpace(p.Notes),
		NextAction:      p.NextAction,
	}
	if p.NextActionDate != nil && *p.NextActionDate != "" {
		d, err := conv.ParseDate(*p.NextActionDate)
		if err != nil {
			response.Fail(c, apperr.Validation("next_action_date must be YYYY-MM-DD"))
			return
		}
		in.NextActionDate = &d
	}

	entry, err := h.repo.Create(c.Request.Context(), c.Param("id"), in, c.GetString(middleware.CtxUserID), c.ClientIP())
	if err != nil {
		if errors.Is(err, ErrRequestNotFound) {
			response.Fail(c, apperr.NotFound("Prayer request"))
			return
		}
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.Created(c, "Follow-up logged", entry)
}

type updatePayload struct {
	ContactDate     *string `json:"contact_date"`
	Channel         *string `json:"channel"`
	UpdateType      *string `json:"update_type"`
	ConditionStatus *string `json:"condition_status"`
	Notes           *string `json:"notes"`
	NextAction      *string `json:"next_action"`
	NextActionDate  *string `json:"next_action_date"`
}

func (h *Handler) update(c *gin.Context) {
	var p updatePayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}

	actorID := c.GetString(middleware.CtxUserID)
	role := c.GetString(middleware.CtxUserRole)
	owner, err := h.repo.Owner(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.NotFound("Follow-up"))
		return
	}
	if role != auth.RoleAdmin && owner != actorID {
		response.Fail(c, apperr.Forbidden("Only the author or an admin can correct this follow-up"))
		return
	}

	in := UpdateInput{
		Channel:         p.Channel,
		UpdateType:      p.UpdateType,
		ConditionStatus: p.ConditionStatus,
		Notes:           p.Notes,
		NextAction:      p.NextAction,
	}
	if p.ContactDate != nil && *p.ContactDate != "" {
		d, err := conv.ParseDate(*p.ContactDate)
		if err != nil {
			response.Fail(c, apperr.Validation("contact_date must be YYYY-MM-DD"))
			return
		}
		in.ContactDate = &d
	}
	if p.NextActionDate != nil && *p.NextActionDate != "" {
		d, err := conv.ParseDate(*p.NextActionDate)
		if err != nil {
			response.Fail(c, apperr.Validation("next_action_date must be YYYY-MM-DD"))
			return
		}
		in.NextActionDate = &d
	}

	entry, err := h.repo.Update(c.Request.Context(), c.Param("id"), in, actorID, c.ClientIP())
	if err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.OK(c, "Follow-up updated", entry)
}

func validChannel(v string) bool {
	return v == "visit" || v == "phone" || v == "whatsapp" || v == "other"
}

func validUpdateType(v string) bool {
	return v == "documentation" || v == "life_update" || v == "current_condition" || v == "other"
}

func validCondition(v string) bool {
	return v == "improving" || v == "stable" || v == "worsening" || v == "answered" || v == "unreachable"
}
