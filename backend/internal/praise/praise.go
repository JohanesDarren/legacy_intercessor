// Package praise implements praise report CRUD (PRD F-04). Praise shares the
// gold identity colour with prayer, so labels and icons must differentiate.
package praise

import (
	"context"
	"errors"
	"fmt"
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

// Report is the API representation of a praise report.
type Report struct {
	ID                 string     `json:"id"`
	ServiceID          *string    `json:"service_id"`
	CategoryID         string     `json:"category_id"`
	CategoryName       string     `json:"category_name"`
	SourceFollowUpID   *string    `json:"source_follow_up_id"`
	ReporterName       *string    `json:"reporter_name"`
	ReporterGender     string     `json:"reporter_gender"`
	Title              string     `json:"title"`
	Content            string     `json:"content"`
	IsFeatured         bool       `json:"is_featured"`
	DisplayConsent     string     `json:"display_consent"`
	ReportedAt         time.Time  `json:"reported_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	CreatedBy          *string    `json:"created_by"`
}

// Filter holds praise list parameters.
type Filter struct {
	Page       int
	Limit      int
	ServiceID  string
	CategoryID string
	Gender     string
	Featured   *bool
	Consent    string
	Search     string
	SortBy     string
	SortOrder  string
	CreatedBy  string
}

// Repository persists praise reports.
type Repository struct {
	pool  *pgxpool.Pool
	audit *audit.Logger
}

// NewRepository builds the repository.
func NewRepository(pool *pgxpool.Pool, auditLog *audit.Logger) *Repository {
	return &Repository{pool: pool, audit: auditLog}
}

const selectColumns = `
	r.id, r.service_id, r.category_id, c.name AS category_name, r.source_follow_up_id,
	r.reporter_name, r.reporter_gender, r.title, r.content, r.is_featured,
	r.display_consent, r.reported_at, r.created_at, r.updated_at, r.created_by
`

const fromClause = `
	FROM praise_reports r
	JOIN categories c ON c.id = r.category_id
`

// List returns a page of praise reports plus the total.
func (r *Repository) List(ctx context.Context, f Filter) ([]Report, int, error) {
	where := []string{"r.deleted_at IS NULL"}
	args := []any{}

	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.ServiceID != "" {
		add("r.service_id = $%d", f.ServiceID)
	}
	if f.CategoryID != "" {
		add("r.category_id = $%d", f.CategoryID)
	}
	if f.Gender != "" {
		add("r.reporter_gender = $%d", f.Gender)
	}
	if f.Featured != nil {
		add("r.is_featured = $%d", *f.Featured)
	}
	if f.Consent != "" {
		add("r.display_consent = $%d", f.Consent)
	}
	if f.CreatedBy != "" {
		add("r.created_by = $%d", f.CreatedBy)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		idx := len(args)
		where = append(where, fmt.Sprintf(
			"(LOWER(r.title) LIKE $%d OR LOWER(COALESCE(r.reporter_name,'')) LIKE $%d OR LOWER(r.content) LIKE $%d)", idx, idx, idx))
	}

	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM praise_reports r`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dir := "DESC"
	if strings.EqualFold(f.SortOrder, "asc") {
		dir = "ASC"
	}
	order := fmt.Sprintf("r.reported_at %s", dir)
	switch f.SortBy {
	case "category":
		order = fmt.Sprintf("c.name %s, r.reported_at DESC", dir)
	case "gender":
		order = fmt.Sprintf("r.reporter_gender %s, r.reported_at DESC", dir)
	case "featured", "is_featured":
		order = fmt.Sprintf("r.is_featured %s, r.reported_at DESC", dir)
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	query := fmt.Sprintf(`SELECT %s %s %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		selectColumns, fromClause, whereSQL, order, len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rep)
	}
	return out, total, rows.Err()
}

// Get returns one report.
func (r *Repository) Get(ctx context.Context, id string) (*Report, error) {
	query := fmt.Sprintf(`SELECT %s %s WHERE r.id = $1 AND r.deleted_at IS NULL`, selectColumns, fromClause)
	rep, err := scanReport(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanReport(s scanner) (*Report, error) {
	var rep Report
	err := s.Scan(
		&rep.ID, &rep.ServiceID, &rep.CategoryID, &rep.CategoryName, &rep.SourceFollowUpID,
		&rep.ReporterName, &rep.ReporterGender, &rep.Title, &rep.Content, &rep.IsFeatured,
		&rep.DisplayConsent, &rep.ReportedAt, &rep.CreatedAt, &rep.UpdatedAt, &rep.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	return &rep, nil
}

// CreateInput carries a validated create payload.
type CreateInput struct {
	ServiceID        *string
	CategoryID       string
	SourceFollowUpID *string
	ReporterName     *string
	ReporterGender   string
	Title            string
	Content          string
	IsFeatured       bool
	DisplayConsent   string
	ReportedAt       *time.Time
}

// Create inserts a praise report.
func (r *Repository) Create(ctx context.Context, in CreateInput, actorID, ip string) (*Report, error) {
	reported := time.Now()
	if in.ReportedAt != nil {
		reported = *in.ReportedAt
	}
	gender := in.ReporterGender
	if gender == "" {
		gender = "unspecified"
	}
	consent := in.DisplayConsent
	if consent == "" {
		consent = "no"
	}

	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO praise_reports
			(service_id, category_id, source_follow_up_id, reporter_name, reporter_gender,
			 title, content, is_featured, display_consent, reported_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
		RETURNING id
	`, in.ServiceID, in.CategoryID, in.SourceFollowUpID, in.ReporterName, gender,
		in.Title, in.Content, in.IsFeatured, consent, reported, actorID).Scan(&id)
	if err != nil {
		return nil, err
	}

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "praise_report.create", Entity: "praise_reports",
		EntityID: id, IP: ip,
	})
	return r.Get(ctx, id)
}

// UpdateInput carries a partial update.
type UpdateInput struct {
	ServiceID      *string
	CategoryID     *string
	ReporterName   *string
	ReporterGender *string
	Title          *string
	Content        *string
	IsFeatured     *bool
	DisplayConsent *string
	ReportedAt     *time.Time
}

// Update patches a praise report.
func (r *Repository) Update(ctx context.Context, id string, in UpdateInput, actorID, ip string) (*Report, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE praise_reports SET
			service_id      = COALESCE($2, service_id),
			category_id     = COALESCE($3, category_id),
			reporter_name   = COALESCE($4, reporter_name),
			reporter_gender = COALESCE($5, reporter_gender),
			title           = COALESCE($6, title),
			content         = COALESCE($7, content),
			is_featured     = COALESCE($8, is_featured),
			display_consent = COALESCE($9, display_consent),
			reported_at     = COALESCE($10, reported_at),
			updated_by      = $11,
			updated_at      = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, in.ServiceID, in.CategoryID, in.ReporterName, in.ReporterGender,
		in.Title, in.Content, in.IsFeatured, in.DisplayConsent, in.ReportedAt, actorID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	fields := []string{}
	if in.CategoryID != nil {
		fields = append(fields, "category_id")
	}
	if in.Title != nil {
		fields = append(fields, "title")
	}
	if in.Content != nil {
		fields = append(fields, "content")
	}
	if in.IsFeatured != nil {
		fields = append(fields, "is_featured")
	}
	if in.DisplayConsent != nil {
		fields = append(fields, "display_consent")
	}
	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "praise_report.update", Entity: "praise_reports",
		EntityID: id, Fields: fields, IP: ip,
	})
	return r.Get(ctx, id)
}

// SoftDelete marks a report deleted.
func (r *Repository) SoftDelete(ctx context.Context, id, actorID, ip string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE praise_reports SET deleted_at = now(), updated_by = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "praise_report.delete", Entity: "praise_reports",
		EntityID: id, IP: ip,
	})
	return nil
}

// Eligible returns reports with display consent for a service.
func (r *Repository) Eligible(ctx context.Context, serviceID string) ([]Report, error) {
	query := fmt.Sprintf(`
		SELECT %s %s
		WHERE r.deleted_at IS NULL AND r.service_id = $1
		  AND r.display_consent IN ('yes', 'anonymous')
		ORDER BY r.is_featured DESC, c.sort_order, r.reported_at DESC
	`, selectColumns, fromClause)

	rows, err := r.pool.Query(ctx, query, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rep)
	}
	return out, rows.Err()
}

// GetMany loads reports by ID.
func (r *Repository) GetMany(ctx context.Context, ids []string) (map[string]*Report, error) {
	if len(ids) == 0 {
		return map[string]*Report{}, nil
	}
	query := fmt.Sprintf(`SELECT %s %s WHERE r.id = ANY($1) AND r.deleted_at IS NULL`, selectColumns, fromClause)
	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]*Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out[rep.ID] = rep
	}
	return out, rows.Err()
}

// ErrNotFound is returned when a report does not exist or is deleted.
var ErrNotFound = errors.New("praise report not found")

// Handler exposes praise endpoints.
type Handler struct{ repo *Repository }

// NewHandler builds the HTTP handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register mounts praise routes.
func (h *Handler) Register(g *gin.RouterGroup) {
	writers := auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor)
	g.GET("/praise-reports", writers, h.list)
	g.GET("/praise-reports/:id", writers, h.get)
	g.POST("/praise-reports", writers, h.create)
	g.PUT("/praise-reports/:id", writers, h.update)
	g.PATCH("/praise-reports/:id", writers, h.update)
	g.DELETE("/praise-reports/:id", auth.RequireRole(auth.RoleAdmin), h.remove)
}

func (h *Handler) list(c *gin.Context) {
	f := Filter{
		Page:       conv.Int(c.Query("page"), 1),
		Limit:      conv.Int(c.Query("limit"), 20),
		ServiceID:  c.Query("service_id"),
		CategoryID: c.Query("category_id"),
		Gender:     c.Query("gender"),
		Consent:    c.Query("consent"),
		Search:     c.Query("q"),
		SortBy:     c.DefaultQuery("sort_by", "reported_at"),
		SortOrder:  c.DefaultQuery("sort_order", "desc"),
		CreatedBy:  c.Query("created_by"),
	}
	if v := c.Query("featured"); v != "" {
		b := v == "true" || v == "1"
		f.Featured = &b
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Page < 1 {
		f.Page = 1
	}

	items, total, err := h.repo.List(c.Request.Context(), f)
	if err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.Paginated(c, "Praise reports retrieved", items, response.NewPageMeta(f.Page, f.Limit, total))
}

func (h *Handler) get(c *gin.Context) {
	rep, err := h.repo.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.OK(c, "Praise report retrieved", rep)
}

type createPayload struct {
	ServiceID        *string `json:"service_id"`
	CategoryID       string  `json:"category_id"`
	SourceFollowUpID *string `json:"source_follow_up_id"`
	ReporterName     *string `json:"reporter_name"`
	ReporterGender   string  `json:"reporter_gender"`
	Title            string  `json:"title"`
	Content          string  `json:"content"`
	IsFeatured       bool    `json:"is_featured"`
	DisplayConsent   string  `json:"display_consent"`
	ReportedAt       *string `json:"reported_at"`
}

func (h *Handler) create(c *gin.Context) {
	var p createPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}
	if fields := validate(p.Title, p.Content, p.CategoryID, p.ReporterGender, p.DisplayConsent); len(fields) > 0 {
		response.Fail(c, apperr.ValidationFields("Please correct the highlighted fields", fields))
		return
	}

	in := CreateInput{
		ServiceID:        p.ServiceID,
		CategoryID:       p.CategoryID,
		SourceFollowUpID: p.SourceFollowUpID,
		ReporterName:     p.ReporterName,
		ReporterGender:   p.ReporterGender,
		Title:            strings.TrimSpace(p.Title),
		Content:          strings.TrimSpace(p.Content),
		IsFeatured:       p.IsFeatured,
		DisplayConsent:   p.DisplayConsent,
	}
	if p.ReportedAt != nil && *p.ReportedAt != "" {
		t, err := conv.ParseTime(*p.ReportedAt)
		if err != nil {
			response.Fail(c, apperr.Validation("reported_at must be an ISO 8601 datetime"))
			return
		}
		in.ReportedAt = &t
	}

	rep, err := h.repo.Create(c.Request.Context(), in, c.GetString(middleware.CtxUserID), c.ClientIP())
	if err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.Created(c, "Praise report created", rep)
}

type updatePayload struct {
	ServiceID      *string `json:"service_id"`
	CategoryID     *string `json:"category_id"`
	ReporterName   *string `json:"reporter_name"`
	ReporterGender *string `json:"reporter_gender"`
	Title          *string `json:"title"`
	Content        *string `json:"content"`
	IsFeatured     *bool   `json:"is_featured"`
	DisplayConsent *string `json:"display_consent"`
	ReportedAt     *string `json:"reported_at"`
}

func (h *Handler) update(c *gin.Context) {
	var p updatePayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}

	in := UpdateInput{
		ServiceID:      p.ServiceID,
		CategoryID:     p.CategoryID,
		ReporterName:   p.ReporterName,
		ReporterGender: p.ReporterGender,
		Title:          p.Title,
		Content:        p.Content,
		IsFeatured:     p.IsFeatured,
		DisplayConsent: p.DisplayConsent,
	}
	if p.ReportedAt != nil && *p.ReportedAt != "" {
		t, err := conv.ParseTime(*p.ReportedAt)
		if err != nil {
			response.Fail(c, apperr.Validation("reported_at must be an ISO 8601 datetime"))
			return
		}
		in.ReportedAt = &t
	}

	rep, err := h.repo.Update(c.Request.Context(), c.Param("id"), in, c.GetString(middleware.CtxUserID), c.ClientIP())
	if err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.OK(c, "Praise report updated", rep)
}

func (h *Handler) remove(c *gin.Context) {
	if err := h.repo.SoftDelete(c.Request.Context(), c.Param("id"), c.GetString(middleware.CtxUserID), c.ClientIP()); err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.OK(c, "Praise report deleted", gin.H{"id": c.Param("id"), "deleted": true})
}

func mapErr(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperr.NotFound("Praise report")
	}
	return apperr.Internal(err)
}

func validate(title, content, categoryID, gender, consent string) map[string][]string {
	f := map[string][]string{}
	if strings.TrimSpace(title) == "" {
		f["title"] = []string{"Title is required"}
	} else if len(title) > 255 {
		f["title"] = []string{"Title must be at most 255 characters"}
	}
	if strings.TrimSpace(content) == "" {
		f["content"] = []string{"Content is required"}
	} else if len(content) > 5000 {
		f["content"] = []string{"Content must be at most 5000 characters"}
	}
	if strings.TrimSpace(categoryID) == "" {
		f["category_id"] = []string{"Category is required"}
	}
	if gender != "" && gender != "male" && gender != "female" && gender != "unspecified" {
		f["reporter_gender"] = []string{"Gender must be male, female or unspecified"}
	}
	if consent != "" && consent != "yes" && consent != "anonymous" && consent != "no" {
		f["display_consent"] = []string{"Display consent must be yes, anonymous or no"}
	}
	return f
}
