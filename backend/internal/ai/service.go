package ai

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/audit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
)

// Summary is a stored AI draft or approved summary.
type Summary struct {
	ID            string     `json:"id"`
	ServiceID     string     `json:"service_id"`
	CategoryID    string     `json:"category_id"`
	CategoryName  string     `json:"category_name"`
	Kind          string     `json:"kind"`
	SourceIDs     []string   `json:"source_ids"`
	AIText        string     `json:"ai_text"`
	FinalText     *string    `json:"final_text"`
	DisplayText   string     `json:"display_text"`
	Status        string     `json:"status"`
	ModelName     string     `json:"model_name"`
	PromptVersion string     `json:"prompt_version"`
	ApprovedBy    *string    `json:"approved_by"`
	ApprovedAt    *time.Time `json:"approved_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// SourceRecord is the record shape the service needs for redaction. It is
// deliberately narrow: no follow-up notes, no phone numbers, no addresses.
type SourceRecord struct {
	ID             string
	CategoryID     string
	CategoryName   string
	Title          string
	Content        string
	Name           string
	DisplayConsent string
}

// Service orchestrates redaction, provider calls and draft storage.
type Service struct {
	pool     *pgxpool.Pool
	provider Provider
	audit    *audit.Logger
	log      *slog.Logger
	timeout  time.Duration
}

// NewService wires the AI service.
func NewService(pool *pgxpool.Pool, provider Provider, auditLog *audit.Logger, log *slog.Logger, timeout time.Duration) *Service {
	return &Service{pool: pool, provider: provider, audit: auditLog, log: log, timeout: timeout}
}

// ProviderName exposes the active provider for the settings screen.
func (s *Service) ProviderName() string { return s.provider.Name() }

// ProviderModel exposes the active model name.
func (s *Service) ProviderModel() string { return s.provider.Model() }

// GenerateInput is the validated request body for POST /ai/summaries.
type GenerateInput struct {
	ServiceID       string `json:"service_id"`
	Kind            string `json:"kind"`
	Language        string `json:"language"`
	MaxWordsPerItem int    `json:"max_words_per_item"`
}

// Generate redacts the eligible records, calls the provider, and stores one
// draft per category. On timeout nothing is saved (PRD F-06).
func (s *Service) Generate(ctx context.Context, in GenerateInput, records []SourceRecord, actorID string) ([]Summary, error) {
	// Defence in depth: drop anything not eligible, even if the caller passed it.
	eligible := make([]SourceRecord, 0, len(records))
	for _, r := range records {
		if EligibleForDisplay(r.DisplayConsent) {
			eligible = append(eligible, r)
		}
	}
	if len(eligible) == 0 {
		return nil, apperr.Unprocessable(apperr.CodeNoEligibleRecords,
			"No records with display consent were found for this service")
	}

	groups := groupByCategory(eligible, in.Kind)
	req := Request{
		Kind:            in.Kind,
		Language:        in.Language,
		MaxWordsPerItem: in.MaxWordsPerItem,
		Groups:          groups,
	}

	callCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	start := time.Now()
	resp, err := s.provider.Summarize(callCtx, req)
	latency := time.Since(start)

	if err != nil {
		// Log counts and latency only, never prompt or output text (PRD 9.3).
		s.log.Warn("ai summary failed",
			slog.String("provider", s.provider.Name()),
			slog.String("model", s.provider.Model()),
			slog.String("prompt_version", PromptVersion),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("result", "error"),
		)
		if errors.Is(err, ErrTimeout) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, apperr.Unprocessable(apperr.CodeAIProviderTimeout,
				"Summary failed: the AI provider did not respond in time. Retry or write the summary manually.")
		}
		return nil, apperr.Unprocessable(apperr.CodeAIUnavailable,
			"The AI provider is unavailable right now. Retry later or write the summary manually.")
	}

	s.log.Info("ai summary generated",
		slog.String("provider", s.provider.Name()),
		slog.String("model", s.provider.Model()),
		slog.String("prompt_version", PromptVersion),
		slog.Int("tokens_in", resp.TokensIn),
		slog.Int("tokens_out", resp.TokensOut),
		slog.Int64("latency_ms", latency.Milliseconds()),
		slog.Int("summaries", len(resp.Summaries)),
		slog.String("result", "ok"),
	)

	return s.store(ctx, in, resp, groups, actorID)
}

// store persists the drafts in one transaction, replacing earlier drafts for
// the same service and kind so the admin never reviews stale duplicates.
func (s *Service) store(ctx context.Context, in GenerateInput, resp *Response, groups []CategoryGroup, actorID string) ([]Summary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE ai_summaries SET deleted_at = now(), updated_at = now()
		WHERE service_id = $1 AND kind = $2 AND status = 'draft' AND deleted_at IS NULL
	`, in.ServiceID, in.Kind); err != nil {
		return nil, apperr.Internal(err)
	}

	ids := make([]string, 0, len(resp.Summaries))
	for _, sum := range resp.Summaries {
		categoryID := sum.CategoryID
		sourceIDs := sum.SourceIDs
		if categoryID == "" {
			for _, g := range groups {
				if strings.EqualFold(g.Category, sum.Category) {
					categoryID = g.CategoryID
					sourceIDs = g.SourceIDs
					break
				}
			}
		}
		if categoryID == "" || len(sourceIDs) == 0 {
			continue
		}

		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO ai_summaries
				(service_id, category_id, kind, source_ids, ai_text, status,
				 model_name, prompt_version, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, 'draft', $6, $7, $8, $8)
			RETURNING id
		`, in.ServiceID, categoryID, in.Kind, sourceIDs, sum.Text,
			resp.ModelName, PromptVersion, actorID).Scan(&id)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, apperr.Unprocessable(apperr.CodeNoEligibleRecords,
			"The AI response did not map to any category. Please retry.")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}

	s.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "ai_summary.generate", Entity: "ai_summaries",
		Fields: []string{"service_id", "kind"},
	})
	return s.ListByService(ctx, in.ServiceID, in.Kind)
}

// groupByCategory builds the redacted provider payload.
func groupByCategory(records []SourceRecord, kind string) []CategoryGroup {
	order := []string{}
	byCat := map[string]*CategoryGroup{}

	for _, r := range records {
		g, ok := byCat[r.CategoryID]
		if !ok {
			g = &CategoryGroup{Category: r.CategoryName, CategoryID: r.CategoryID}
			byCat[r.CategoryID] = g
			order = append(order, r.CategoryID)
		}
		g.Items = append(g.Items, Item{
			DisplayName: RedactName(r.Name, r.DisplayConsent),
			Category:    r.CategoryName,
			Title:       RedactText(r.Title),
			Content:     RedactText(r.Content),
		})
		g.SourceIDs = append(g.SourceIDs, r.ID)
	}

	out := make([]CategoryGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *byCat[id])
	}
	return out
}

// ListByService returns all live summaries for a service and kind.
func (s *Service) ListByService(ctx context.Context, serviceID, kind string) ([]Summary, error) {
	q := `
		SELECT a.id, a.service_id, a.category_id, c.name, a.kind, a.source_ids,
		       a.ai_text, a.final_text, a.status, a.model_name, a.prompt_version,
		       a.approved_by, a.approved_at, a.created_at, a.updated_at
		FROM ai_summaries a
		JOIN categories c ON c.id = a.category_id
		WHERE a.service_id = $1 AND a.deleted_at IS NULL`
	args := []any{serviceID}
	if kind != "" {
		q += ` AND a.kind = $2`
		args = append(args, kind)
	}
	q += ` ORDER BY c.sort_order, c.name`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []Summary{}
	for rows.Next() {
		var sm Summary
		if err := rows.Scan(&sm.ID, &sm.ServiceID, &sm.CategoryID, &sm.CategoryName, &sm.Kind,
			&sm.SourceIDs, &sm.AIText, &sm.FinalText, &sm.Status, &sm.ModelName,
			&sm.PromptVersion, &sm.ApprovedBy, &sm.ApprovedAt, &sm.CreatedAt, &sm.UpdatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		if sm.FinalText != nil {
			sm.DisplayText = *sm.FinalText
		} else {
			sm.DisplayText = sm.AIText
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

// Get returns one summary.
func (s *Service) Get(ctx context.Context, id string) (*Summary, error) {
	var sm Summary
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.service_id, a.category_id, c.name, a.kind, a.source_ids,
		       a.ai_text, a.final_text, a.status, a.model_name, a.prompt_version,
		       a.approved_by, a.approved_at, a.created_at, a.updated_at
		FROM ai_summaries a
		JOIN categories c ON c.id = a.category_id
		WHERE a.id = $1 AND a.deleted_at IS NULL
	`, id).Scan(&sm.ID, &sm.ServiceID, &sm.CategoryID, &sm.CategoryName, &sm.Kind,
		&sm.SourceIDs, &sm.AIText, &sm.FinalText, &sm.Status, &sm.ModelName,
		&sm.PromptVersion, &sm.ApprovedBy, &sm.ApprovedAt, &sm.CreatedAt, &sm.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Summary")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if sm.FinalText != nil {
		sm.DisplayText = *sm.FinalText
	} else {
		sm.DisplayText = sm.AIText
	}
	return &sm, nil
}

// ReviewInput carries an edit, approval or rejection.
type ReviewInput struct {
	FinalText *string `json:"final_text"`
	Status    *string `json:"status"`
}

// Review applies a human decision. Approving stores the human text in
// final_text and keeps ai_text untouched for audit (PRD F-06).
func (s *Service) Review(ctx context.Context, id string, in ReviewInput, actorID string) (*Summary, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	status := current.Status
	if in.Status != nil {
		switch *in.Status {
		case "draft", "approved", "rejected":
			status = *in.Status
		default:
			return nil, apperr.Validation("Status must be draft, approved or rejected")
		}
	}

	finalText := current.FinalText
	if in.FinalText != nil {
		t := strings.TrimSpace(*in.FinalText)
		if t == "" {
			return nil, apperr.Validation("Summary text cannot be empty")
		}
		finalText = &t
	}

	// Approving without edits keeps the AI text as the final text.
	if status == "approved" && finalText == nil {
		t := current.AIText
		finalText = &t
	}
	if status == "approved" && finalText != nil && strings.TrimSpace(*finalText) == "" {
		return nil, apperr.Validation("Cannot approve an empty summary")
	}

	var approvedBy *string
	var approvedAt *time.Time
	if status == "approved" {
		approvedBy = &actorID
		now := time.Now()
		approvedAt = &now
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE ai_summaries
		SET final_text = $2, status = $3, approved_by = $4, approved_at = $5,
		    updated_by = $6, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, finalText, status, approvedBy, approvedAt, actorID); err != nil {
		return nil, apperr.Internal(err)
	}

	s.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "ai_summary." + status, Entity: "ai_summaries",
		EntityID: id, Fields: []string{"final_text", "status"},
	})
	return s.Get(ctx, id)
}

// UnapprovedCount returns how many live summaries for a service and kind are
// still drafts, used to block exports (T-04).
func (s *Service) UnapprovedCount(ctx context.Context, serviceID, kind string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_summaries
		WHERE service_id = $1 AND kind = $2 AND status <> 'approved' AND deleted_at IS NULL
	`, serviceID, kind).Scan(&n)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return n, nil
}

// ApprovedFor returns approved summaries keyed by category, used by the
// presentation builder.
func (s *Service) ApprovedFor(ctx context.Context, serviceID, kind string) (map[string]*Summary, error) {
	items, err := s.ListByService(ctx, serviceID, kind)
	if err != nil {
		return nil, err
	}
	out := map[string]*Summary{}
	for i := range items {
		if items[i].Status == "approved" {
			out[items[i].CategoryID] = &items[i]
		}
	}
	return out, nil
}
