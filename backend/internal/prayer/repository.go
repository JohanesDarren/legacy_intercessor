// Package prayer implements prayer request CRUD with sorting, filtering,
// search, duplicate warnings and the consent rules from PRD F-03 / F-05.
package prayer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/audit"
)

// Request is the API representation of a prayer request.
type Request struct {
	ID             string     `json:"id"`
	ServiceID      *string    `json:"service_id"`
	CategoryID     string     `json:"category_id"`
	CategoryName   string     `json:"category_name"`
	RequesterName  *string    `json:"requester_name"`
	RequesterGender string    `json:"requester_gender"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	IsUrgent       bool       `json:"is_urgent"`
	Status         string     `json:"status"`
	DisplayConsent string     `json:"display_consent"`
	ReceivedAt     time.Time  `json:"received_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CreatedBy      *string    `json:"created_by"`
	// FollowUpCount and LastFollowUpAt drive the dashboard reminder (F-02/F-08).
	FollowUpCount  int        `json:"follow_up_count"`
	LastFollowUpAt *time.Time `json:"last_follow_up_at"`
}

// Filter holds list query parameters (PRD GET /prayer-requests).
type Filter struct {
	Page       int
	Limit      int
	ServiceID  string
	CategoryID string
	Gender     string
	Urgent     *bool
	Status     string
	Consent    string
	Search     string
	SortBy     string
	SortOrder  string
	// CreatedBy limits results to one author (used by the dashboard).
	CreatedBy string
}

// Repository persists prayer requests.
type Repository struct {
	pool  *pgxpool.Pool
	audit *audit.Logger
}

// NewRepository builds the repository.
func NewRepository(pool *pgxpool.Pool, auditLog *audit.Logger) *Repository {
	return &Repository{pool: pool, audit: auditLog}
}

const selectColumns = `
	p.id, p.service_id, p.category_id, c.name AS category_name,
	p.requester_name, p.requester_gender, p.title, p.content,
	p.is_urgent, p.status, p.display_consent, p.received_at,
	p.created_at, p.updated_at, p.created_by,
	COALESCE(f.follow_up_count, 0) AS follow_up_count, f.last_follow_up_at
`

const fromClause = `
	FROM prayer_requests p
	JOIN categories c ON c.id = p.category_id
	LEFT JOIN (
		SELECT prayer_request_id, COUNT(*) AS follow_up_count, MAX(created_at) AS last_follow_up_at
		FROM follow_ups WHERE deleted_at IS NULL GROUP BY prayer_request_id
	) f ON f.prayer_request_id = p.id
`

// List returns a page of prayer requests plus the total count.
func (r *Repository) List(ctx context.Context, f Filter) ([]Request, int, error) {
	where := []string{"p.deleted_at IS NULL"}
	args := []any{}

	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.ServiceID != "" {
		add("p.service_id = $%d", f.ServiceID)
	}
	if f.CategoryID != "" {
		add("p.category_id = $%d", f.CategoryID)
	}
	if f.Gender != "" {
		add("p.requester_gender = $%d", f.Gender)
	}
	if f.Urgent != nil {
		add("p.is_urgent = $%d", *f.Urgent)
	}
	if f.Status != "" {
		add("p.status = $%d", f.Status)
	}
	if f.Consent != "" {
		add("p.display_consent = $%d", f.Consent)
	}
	if f.CreatedBy != "" {
		add("p.created_by = $%d", f.CreatedBy)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		idx := len(args)
		where = append(where, fmt.Sprintf(
			"(LOWER(p.title) LIKE $%d OR LOWER(COALESCE(p.requester_name,'')) LIKE $%d OR LOWER(p.content) LIKE $%d)", idx, idx, idx))
	}

	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM prayer_requests p`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := orderClause(f.SortBy, f.SortOrder)
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	query := fmt.Sprintf(`SELECT %s %s %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		selectColumns, fromClause, whereSQL, order, len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Request{}
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *req)
	}
	return out, total, rows.Err()
}

// orderClause maps the API sort parameters to SQL. Urgent sorting puts urgent
// rows first, then newest (PRD GET /prayer-requests notes).
func orderClause(sortBy, sortOrder string) string {
	dir := "DESC"
	if strings.EqualFold(sortOrder, "asc") {
		dir = "ASC"
	}
	switch sortBy {
	case "category":
		return fmt.Sprintf("c.name %s, p.received_at DESC", dir)
	case "gender":
		return fmt.Sprintf("p.requester_gender %s, p.received_at DESC", dir)
	case "is_urgent", "priority":
		return fmt.Sprintf("p.is_urgent %s, p.received_at DESC", dir)
	default:
		return fmt.Sprintf("p.received_at %s", dir)
	}
}

// Get returns one request by ID.
func (r *Repository) Get(ctx context.Context, id string) (*Request, error) {
	query := fmt.Sprintf(`SELECT %s %s WHERE p.id = $1 AND p.deleted_at IS NULL`, selectColumns, fromClause)
	row := r.pool.QueryRow(ctx, query, id)
	req, err := scanRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return req, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRequest(s scanner) (*Request, error) {
	var req Request
	err := s.Scan(
		&req.ID, &req.ServiceID, &req.CategoryID, &req.CategoryName,
		&req.RequesterName, &req.RequesterGender, &req.Title, &req.Content,
		&req.IsUrgent, &req.Status, &req.DisplayConsent, &req.ReceivedAt,
		&req.CreatedAt, &req.UpdatedAt, &req.CreatedBy,
		&req.FollowUpCount, &req.LastFollowUpAt,
	)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// CreateInput carries validated create payload.
type CreateInput struct {
	ServiceID       *string
	CategoryID      string
	RequesterName   *string
	RequesterGender string
	Title           string
	Content         string
	IsUrgent        bool
	DisplayConsent  string
	Status          string
	ReceivedAt      *time.Time
	ConfirmDuplicate bool
}

// Create inserts a prayer request, returning a duplicate warning when a very
// similar request from the same requester exists within 7 days (PRD F-03).
func (r *Repository) Create(ctx context.Context, in CreateInput, actorID, ip string) (*Request, error) {
	if !in.ConfirmDuplicate && in.RequesterName != nil && strings.TrimSpace(*in.RequesterName) != "" {
		dup, err := r.findDuplicate(ctx, *in.RequesterName, in.Title)
		if err != nil {
			return nil, err
		}
		if dup {
			return nil, ErrDuplicate
		}
	}

	received := time.Now()
	if in.ReceivedAt != nil {
		received = *in.ReceivedAt
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	gender := in.RequesterGender
	if gender == "" {
		gender = "unspecified"
	}
	consent := in.DisplayConsent
	if consent == "" {
		consent = "no"
	}

	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO prayer_requests
			(service_id, category_id, requester_name, requester_gender, title, content,
			 is_urgent, status, display_consent, received_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
		RETURNING id
	`, in.ServiceID, in.CategoryID, in.RequesterName, gender, in.Title, in.Content,
		in.IsUrgent, status, consent, received, actorID).Scan(&id)
	if err != nil {
		return nil, err
	}

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "prayer_request.create", Entity: "prayer_requests",
		EntityID: id, IP: ip,
	})
	return r.Get(ctx, id)
}

// findDuplicate reports whether a similar request exists in the last 7 days.
func (r *Repository) findDuplicate(ctx context.Context, requesterName, title string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM prayer_requests
			WHERE deleted_at IS NULL
			  AND LOWER(requester_name) = LOWER($1)
			  AND similarity(LOWER(title), LOWER($2)) > 0.45
			  AND created_at > now() - interval '7 days'
		)
	`, requesterName, title).Scan(&exists)
	if err != nil {
		// pg_trgm may be unavailable; fall back to a LIKE comparison.
		err2 := r.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM prayer_requests
				WHERE deleted_at IS NULL
				  AND LOWER(requester_name) = LOWER($1)
				  AND LOWER(title) LIKE '%' || LOWER($2) || '%'
				  AND created_at > now() - interval '7 days'
			)
		`, requesterName, title).Scan(&exists)
		if err2 != nil {
			return false, err2
		}
	}
	return exists, nil
}

// UpdateInput carries a partial update. Nil fields are left unchanged.
type UpdateInput struct {
	ServiceID       *string
	CategoryID      *string
	RequesterName   *string
	RequesterGender *string
	Title           *string
	Content         *string
	IsUrgent        *bool
	Status          *string
	DisplayConsent  *string
	ReceivedAt      *time.Time
}

// Update patches a request and records the changed field names.
func (r *Repository) Update(ctx context.Context, id string, in UpdateInput, actorID, ip string) (*Request, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE prayer_requests SET
			service_id       = COALESCE($2, service_id),
			category_id      = COALESCE($3, category_id),
			requester_name   = COALESCE($4, requester_name),
			requester_gender = COALESCE($5, requester_gender),
			title            = COALESCE($6, title),
			content          = COALESCE($7, content),
			is_urgent        = COALESCE($8, is_urgent),
			status           = COALESCE($9, status),
			display_consent  = COALESCE($10, display_consent),
			received_at      = COALESCE($11, received_at),
			updated_by       = $12,
			updated_at       = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, in.ServiceID, in.CategoryID, in.RequesterName, in.RequesterGender,
		in.Title, in.Content, in.IsUrgent, in.Status, in.DisplayConsent, in.ReceivedAt, actorID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "prayer_request.update", Entity: "prayer_requests",
		EntityID: id, Fields: changedFields(in), IP: ip,
	})
	return r.Get(ctx, id)
}

// changedFields lists column names only, never values (PRD 9.3).
func changedFields(in UpdateInput) []string {
	var out []string
	if in.ServiceID != nil {
		out = append(out, "service_id")
	}
	if in.CategoryID != nil {
		out = append(out, "category_id")
	}
	if in.RequesterName != nil {
		out = append(out, "requester_name")
	}
	if in.RequesterGender != nil {
		out = append(out, "requester_gender")
	}
	if in.Title != nil {
		out = append(out, "title")
	}
	if in.Content != nil {
		out = append(out, "content")
	}
	if in.IsUrgent != nil {
		out = append(out, "is_urgent")
	}
	if in.Status != nil {
		out = append(out, "status")
	}
	if in.DisplayConsent != nil {
		out = append(out, "display_consent")
	}
	if in.ReceivedAt != nil {
		out = append(out, "received_at")
	}
	return out
}

// SoftDelete marks a request deleted. Hard deletes never run on business tables.
func (r *Repository) SoftDelete(ctx context.Context, id, actorID, ip string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE prayer_requests SET deleted_at = now(), updated_by = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "prayer_request.delete", Entity: "prayer_requests",
		EntityID: id, IP: ip,
	})
	return nil
}

// Eligible returns requests with display consent for a service and kind, used
// by the AI summary and slide builder. Follow-up notes are never included.
func (r *Repository) Eligible(ctx context.Context, serviceID string) ([]Request, error) {
	query := fmt.Sprintf(`
		SELECT %s %s
		WHERE p.deleted_at IS NULL AND p.service_id = $1
		  AND p.display_consent IN ('yes', 'anonymous')
		ORDER BY p.is_urgent DESC, c.sort_order, p.received_at DESC
	`, selectColumns, fromClause)

	rows, err := r.pool.Query(ctx, query, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Request{}
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *req)
	}
	return out, rows.Err()
}

// GetMany loads requests by ID, preserving the caller's order.
func (r *Repository) GetMany(ctx context.Context, ids []string) (map[string]*Request, error) {
	if len(ids) == 0 {
		return map[string]*Request{}, nil
	}
	query := fmt.Sprintf(`SELECT %s %s WHERE p.id = ANY($1) AND p.deleted_at IS NULL`, selectColumns, fromClause)
	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]*Request{}
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out[req.ID] = req
	}
	return out, rows.Err()
}

// Errors returned by the repository, mapped to AppErrors in the service layer.
var (
	ErrNotFound  = errors.New("prayer request not found")
	ErrDuplicate = errors.New("possible duplicate prayer request")
)
