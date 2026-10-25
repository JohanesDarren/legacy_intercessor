package ai

import (
	"context"
	"fmt"
	"strings"
)

// Offline is the deterministic provider used when no LLM credentials are
// configured. It composes a plain, factual draft from the record titles so the
// app works end to end without an AI account. The output is still stored as an
// unapproved draft, so the human review gate is unchanged.
type Offline struct{}

// NewOffline builds the offline provider.
func NewOffline() Provider { return &Offline{} }

// Name identifies the provider.
func (o *Offline) Name() string { return "offline" }

// Model returns the pseudo model name recorded on drafts.
func (o *Offline) Model() string { return "offline-draft-v1" }

// Summarize builds one draft per category from the record titles. Titles are
// de-duplicated and the count reflects the number of distinct items, so the
// draft reads sensibly even when several records share a heading.
func (o *Offline) Summarize(ctx context.Context, req Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, wrapProviderError(ctx, err)
	}

	out := make([]CategorySummary, 0, len(req.Groups))
	for _, g := range req.Groups {
		seen := map[string]bool{}
		titles := make([]string, 0, len(g.Items))
		for _, it := range g.Items {
			t := strings.TrimSpace(it.Title)
			if t == "" {
				continue
			}
			key := strings.ToLower(t)
			if seen[key] {
				continue
			}
			seen[key] = true
			titles = append(titles, lowerFirst(t))
		}

		var text string
		switch {
		case len(titles) == 0:
			if req.Kind == "praise" {
				text = fmt.Sprintf("We give thanks for %d testimony(ies) in %s.", len(g.Items), g.Category)
			} else {
				text = fmt.Sprintf("Please pray for %d request(s) in %s.", len(g.Items), g.Category)
			}
		case req.Kind == "praise":
			text = fmt.Sprintf("We give thanks for %d testimony(ies) in %s: %s.",
				len(g.Items), g.Category, strings.Join(titles, "; "))
		default:
			text = fmt.Sprintf("Please pray for %d request(s) in %s: %s.",
				len(g.Items), g.Category, strings.Join(titles, "; "))
		}

		out = append(out, CategorySummary{
			Category:   g.Category,
			CategoryID: g.CategoryID,
			Text:       text,
			SourceIDs:  g.SourceIDs,
		})
	}

	return &Response{Summaries: out, ModelName: o.Model()}, nil
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] = r[0] - 'A' + 'a'
	}
	return string(r)
}
