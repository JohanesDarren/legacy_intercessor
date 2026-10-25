package presentation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/ai"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/audit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/storage"
)

// RecordSource is the narrow view of a record the builder needs. Implemented
// by the prayer and praise repositories.
type RecordSource struct {
	ID             string
	CategoryID     string
	CategoryName   string
	Title          string
	Content        string
	Name           string
	DisplayConsent string
	IsUrgent       bool
	IsFeatured     bool
}

// Builder assembles slides from approved summaries and eligible records.
type Builder struct {
	// RecordsFor returns eligible records for a service, keyed by kind.
	RecordsFor func(ctx context.Context, serviceID, kind string) ([]RecordSource, error)
	// ApprovedSummaryFor returns the approved summary text per category.
	ApprovedSummaryFor func(ctx context.Context, serviceID, kind string) (map[string]string, error)
	// DraftCount returns how many summaries are still drafts.
	DraftCount func(ctx context.Context, serviceID, kind string) (int, error)
}

// Service manages presentations and export jobs.
type Service struct {
	pool     *pgxpool.Pool
	renderer *Renderer
	store    storage.Store
	builder  *Builder
	audit    *audit.Logger
	log      *slog.Logger
	baseURL  string
	// exportSem bounds concurrent Chromium runs (R-05: one at a time on 2 GB).
	exportSem chan struct{}
}

// NewService wires the presentation service.
func NewService(pool *pgxpool.Pool, renderer *Renderer, store storage.Store, builder *Builder,
	auditLog *audit.Logger, log *slog.Logger, baseURL string, maxConcurrent int) *Service {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{
		pool: pool, renderer: renderer, store: store, builder: builder,
		audit: auditLog, log: log, baseURL: strings.TrimRight(baseURL, "/"),
		exportSem: make(chan struct{}, maxConcurrent),
	}
}

// BuildInput requests a new presentation set.
type BuildInput struct {
	ServiceID string `json:"service_id"`
	Kind      string `json:"kind"`
	Note      string `json:"note"`
}

// Build creates a presentation from approved summaries grouped by category.
// Each category becomes one or more slides according to the template limit.
func (s *Service) Build(ctx context.Context, in BuildInput, actorID string) (*Presentation, error) {
	records, err := s.builder.RecordsFor(ctx, in.ServiceID, in.Kind)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, apperr.Unprocessable(apperr.CodeEmptyPresentation,
			"No eligible records with display consent exist for this service")
	}

	summaries, err := s.builder.ApprovedSummaryFor(ctx, in.ServiceID, in.Kind)
	if err != nil {
		return nil, err
	}

	cfg := ConfigFor(in.Kind)
	slides := assembleSlides(in.Kind, cfg, records, summaries)

	var version int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1 FROM presentations
		WHERE service_id = $1 AND kind = $2
	`, in.ServiceID, in.Kind).Scan(&version); err != nil {
		return nil, apperr.Internal(err)
	}

	slidesJSON, err := json.Marshal(slides)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO presentations (service_id, kind, version, status, slides, created_by, updated_by)
		VALUES ($1, $2, $3, 'draft', $4, $5, $5)
		RETURNING id
	`, in.ServiceID, in.Kind, version, slidesJSON, actorID).Scan(&id)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	s.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "presentation.create", Entity: "presentations", EntityID: id,
		Fields: []string{"service_id", "kind", "slides"},
	})
	return s.Get(ctx, id)
}

// assembleSlides groups records by category, paginates them, and attaches the
// approved summary text as the slide subtitle.
func assembleSlides(kind string, cfg TemplateConfig, records []RecordSource, summaries map[string]string) []Slide {
	order := []string{}
	byCat := map[string][]RecordSource{}
	for _, r := range records {
		if _, ok := byCat[r.CategoryID]; !ok {
			order = append(order, r.CategoryID)
		}
		byCat[r.CategoryID] = append(byCat[r.CategoryID], r)
	}

	slides := []Slide{}
	for _, catID := range order {
		group := byCat[catID]
		title := group[0].CategoryName

		for start := 0; start < len(group); start += cfg.ItemsPerSlide {
			end := start + cfg.ItemsPerSlide
			if end > len(group) {
				end = len(group)
			}
			chunk := group[start:end]

			items := make([]SlideItem, 0, len(chunk))
			for _, r := range chunk {
				items = append(items, SlideItem{
					RefID: r.ID,
					Kind:  kind,
					Title: ai.RedactText(r.Title),
					// Contact details are stripped here as well as in the AI
					// path: a phone number must never reach the videotron.
					Body:     ai.RedactText(summarizeBody(r.Content)),
					Name:     displayName(r.Name, r.DisplayConsent),
					IsUrgent: kind == "prayer" && r.IsUrgent,
				})
			}

			slide := Slide{
				ID:       fmt.Sprintf("%s-%d-%d", kind, len(slides)+1, time.Now().UnixNano()%100000),
				Template: kind,
				Title:    title,
				Items:    items,
			}
			if text, ok := summaries[catID]; ok && strings.TrimSpace(text) != "" {
				slide.Note = strings.TrimSpace(text)
			}
			slides = append(slides, slide)
		}
	}
	return slides
}

// summarizeBody keeps slide text short enough to stay readable at 36px.
func summarizeBody(content string) string {
	content = strings.TrimSpace(content)
	const limit = 220
	if len([]rune(content)) <= limit {
		return content
	}
	runes := []rune(content)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// displayName returns "" for anonymous records so no name reaches the slide.
func displayName(name, consent string) string {
	if consent != "yes" {
		return ""
	}
	return strings.TrimSpace(name)
}

// Get loads one presentation.
func (s *Service) Get(ctx context.Context, id string) (*Presentation, error) {
	var (
		p      Presentation
		slides []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, service_id, kind, version, status, slides, created_at, updated_at, created_by
		FROM presentations WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&p.ID, &p.ServiceID, &p.Kind, &p.Version, &p.Status, &slides,
		&p.CreatedAt, &p.UpdatedAt, &p.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Presentation")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if err := json.Unmarshal(slides, &p.Slides); err != nil {
		return nil, apperr.Internal(err)
	}
	return &p, nil
}

// List returns presentations for a service, newest version first.
func (s *Service) List(ctx context.Context, serviceID, kind string) ([]Presentation, error) {
	q := `SELECT id, service_id, kind, version, status, slides, created_at, updated_at, created_by
	      FROM presentations WHERE deleted_at IS NULL`
	args := []any{}
	if serviceID != "" {
		args = append(args, serviceID)
		q += fmt.Sprintf(" AND service_id = $%d", len(args))
	}
	if kind != "" {
		args = append(args, kind)
		q += fmt.Sprintf(" AND kind = $%d", len(args))
	}
	q += " ORDER BY created_at DESC, version DESC LIMIT 50"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []Presentation{}
	for rows.Next() {
		var (
			p      Presentation
			slides []byte
		)
		if err := rows.Scan(&p.ID, &p.ServiceID, &p.Kind, &p.Version, &p.Status, &slides,
			&p.CreatedAt, &p.UpdatedAt, &p.CreatedBy); err != nil {
			return nil, apperr.Internal(err)
		}
		_ = json.Unmarshal(slides, &p.Slides)
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateInput allows reordering, removing slides and text overrides without
// touching the source records (PRD F-07).
type UpdateInput struct {
	Slides *[]Slide `json:"slides"`
	Note   *string  `json:"note"`
}

// Update replaces the slide list.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Presentation, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Slides != nil {
		p.Slides = *in.Slides
	}
	raw, err := json.Marshal(p.Slides)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE presentations SET slides = $2, updated_by = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, raw, actorID); err != nil {
		return nil, apperr.Internal(err)
	}
	s.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "presentation.update", Entity: "presentations", EntityID: id,
		Fields: []string{"slides"},
	})
	return s.Get(ctx, id)
}

// validateExport enforces the two hard blocks: consent and summary approval
// (T-03, T-04).
func (s *Service) validateExport(ctx context.Context, p *Presentation) error {
	if len(p.Slides) == 0 {
		return apperr.Unprocessable(apperr.CodeEmptyPresentation, "There are no slides to export")
	}

	records, err := s.builder.RecordsFor(ctx, p.ServiceID, p.Kind)
	if err != nil {
		return err
	}
	consent := make(map[string]string, len(records))
	for _, r := range records {
		consent[r.ID] = r.DisplayConsent
	}

	for _, slide := range p.Slides {
		for _, item := range slide.Items {
			if c, ok := consent[item.RefID]; !ok || (c != "yes" && c != "anonymous") {
				return apperr.Unprocessable(apperr.CodeConsentMissing,
					"A slide includes a record without display consent. Remove it or update the consent first.")
			}
		}
	}

	drafts, err := s.builder.DraftCount(ctx, p.ServiceID, p.Kind)
	if err != nil {
		return err
	}
	if drafts > 0 {
		return apperr.Unprocessable(apperr.CodeSummaryNotApprove,
			"A summary is still a draft. Approve every summary before exporting.")
	}
	return nil
}

// StartExport creates the job row and runs the render in the background.
func (s *Service) StartExport(ctx context.Context, presentationID, format, actorID string) (*Export, error) {
	if format != "png" && format != "pdf" {
		return nil, apperr.Validation("format must be png or pdf")
	}
	p, err := s.Get(ctx, presentationID)
	if err != nil {
		return nil, err
	}
	if err := s.validateExport(ctx, p); err != nil {
		return nil, err
	}

	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO presentation_exports (presentation_id, format, status, created_by)
		VALUES ($1, $2, 'queued', $3)
		RETURNING id
	`, presentationID, format, actorID).Scan(&id)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	// Detach from the request context: the job outlives the HTTP response.
	go s.runExport(context.WithoutCancel(ctx), id, p, format)

	return s.GetExport(ctx, id)
}

// runExport renders the slides and stores the artefact.
func (s *Service) runExport(ctx context.Context, exportID string, p *Presentation, format string) {
	s.exportSem <- struct{}{}
	defer func() { <-s.exportSem }()

	started := time.Now()
	if _, err := s.pool.Exec(ctx, `UPDATE presentation_exports SET status = 'running' WHERE id = $1`, exportID); err != nil {
		s.log.Error("export status update failed", slog.String("export_id", exportID), slog.Any("error", err))
		return
	}

	note := ""
	if len(p.Slides) > 0 {
		note = p.Slides[0].Note
	}

	fail := func(err error) {
		msg := "Export failed"
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "Export timed out while rendering"
		}
		// Technical message only, never slide text (PRD 6.3).
		_, _ = s.pool.Exec(ctx, `
			UPDATE presentation_exports SET status = 'failed', error_message = $2 WHERE id = $1
		`, exportID, msg)
		s.log.Error("export failed",
			slog.String("export_id", exportID),
			slog.String("format", format),
			slog.Int("slides", len(p.Slides)),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()),
			slog.Any("error", err),
		)
	}

	var (
		data        []byte
		contentType string
		ext         string
	)

	switch format {
	case "pdf":
		pdf, err := s.renderer.RenderPDF(ctx, p.Slides, note)
		if err != nil {
			fail(err)
			return
		}
		data, contentType, ext = pdf, "application/pdf", "pdf"
	default:
		pngs, err := s.renderer.RenderPNG(ctx, p.Slides, note)
		if err != nil {
			fail(err)
			return
		}
		zipped, err := ZipPNGs(pngs)
		if err != nil {
			fail(err)
			return
		}
		data, contentType, ext = zipped, "application/zip", "zip"
	}

	key := fmt.Sprintf("%s/%s/v%d-%s.%s", p.ServiceID, p.Kind, p.Version, exportID[:8], ext)
	if err := s.store.Put(ctx, key, bytes.NewReader(data), contentType); err != nil {
		fail(err)
		return
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE presentation_exports SET status = 'done', storage_key = $2 WHERE id = $1
	`, exportID, key); err != nil {
		s.log.Error("export finalise failed", slog.String("export_id", exportID), slog.Any("error", err))
		return
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE presentations SET status = 'exported', updated_at = now() WHERE id = $1
	`, p.ID); err != nil {
		s.log.Warn("presentation status update failed", slog.String("presentation_id", p.ID), slog.Any("error", err))
	}

	s.log.Info("export completed",
		slog.String("export_id", exportID),
		slog.String("format", format),
		slog.Int("slides", len(p.Slides)),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		slog.String("result", "done"),
	)
	s.audit.Record(ctx, audit.Entry{
		Action: "presentation.export", Entity: "presentation_exports", EntityID: exportID,
		Fields: []string{"format", "storage_key"},
	})
}

// GetExport returns one export job with a download link when ready.
func (s *Service) GetExport(ctx context.Context, id string) (*Export, error) {
	var e Export
	var key *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, presentation_id, format, status, storage_key, error_message, created_at
		FROM presentation_exports WHERE id = $1
	`, id).Scan(&e.ID, &e.PresentationID, &e.Format, &e.Status, &key, &e.ErrorMessage, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Export")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	e.StorageKey = key
	if e.Status == "done" && key != nil {
		url, err := s.store.URL(ctx, *key, 10*time.Minute)
		if err == nil {
			e.DownloadURL = &url
		}
	}
	return &e, nil
}

// ListExports returns the export history for a presentation, newest first.
func (s *Service) ListExports(ctx context.Context, presentationID string) ([]Export, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, presentation_id, format, status, storage_key, error_message, created_at
		FROM presentation_exports WHERE presentation_id = $1
		ORDER BY created_at DESC LIMIT 50
	`, presentationID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []Export{}
	for rows.Next() {
		var e Export
		if err := rows.Scan(&e.ID, &e.PresentationID, &e.Format, &e.Status, &e.StorageKey,
			&e.ErrorMessage, &e.CreatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		if e.Status == "done" && e.StorageKey != nil {
			if url, err := s.store.URL(ctx, *e.StorageKey, 10*time.Minute); err == nil {
				e.DownloadURL = &url
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestApprovedExport returns the newest completed export for a service and
// kind, which is what the media team downloads (US-15).
func (s *Service) LatestApprovedExport(ctx context.Context, serviceID, kind string) (*Export, error) {
	var e Export
	var key *string
	err := s.pool.QueryRow(ctx, `
		SELECT e.id, e.presentation_id, e.format, e.status, e.storage_key, e.error_message, e.created_at
		FROM presentation_exports e
		JOIN presentations p ON p.id = e.presentation_id
		WHERE p.service_id = $1 AND p.kind = $2 AND e.status = 'done' AND p.deleted_at IS NULL
		ORDER BY e.created_at DESC LIMIT 1
	`, serviceID, kind).Scan(&e.ID, &e.PresentationID, &e.Format, &e.Status, &key, &e.ErrorMessage, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Export")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	e.StorageKey = key
	if key != nil {
		if url, err := s.store.URL(ctx, *key, 10*time.Minute); err == nil {
			e.DownloadURL = &url
		}
	}
	return &e, nil
}
