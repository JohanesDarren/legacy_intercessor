// Package ai implements the AI summary feature (PRD F-06). Everything that
// leaves the system passes through Redact first; nothing with consent "no"
// ever reaches a provider.
package ai

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PromptVersion is stored with every summary for audit (PRD F-06).
const PromptVersion = "v1.0"

// Item is the minimal, already-redacted payload sent to the provider.
type Item struct {
	// DisplayName is "Person A" for anonymous records, the real name only when
	// consent is "yes", and empty when no name was given.
	DisplayName string `json:"name,omitempty"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Content     string `json:"content"`
}

// CategoryGroup is a set of items in one category.
type CategoryGroup struct {
	Category   string   `json:"category"`
	CategoryID string   `json:"category_id"`
	Items      []Item   `json:"items"`
	SourceIDs  []string `json:"source_ids"`
}

// Request is the provider-agnostic summary request.
type Request struct {
	Kind            string          `json:"kind"`
	Language        string          `json:"language"`
	MaxWordsPerItem int             `json:"max_words_per_item"`
	Groups          []CategoryGroup `json:"groups"`
}

// Response is the provider-agnostic summary response.
type Response struct {
	Summaries  []CategorySummary `json:"summaries"`
	ModelName  string            `json:"model_name"`
	TokensIn   int               `json:"tokens_in"`
	TokensOut  int               `json:"tokens_out"`
	LatencyMS  int64             `json:"latency_ms"`
}

// CategorySummary is one drafted summary.
type CategorySummary struct {
	Category  string   `json:"category"`
	CategoryID string  `json:"category_id"`
	Text      string   `json:"text"`
	SourceIDs []string `json:"source_ids"`
}

// Provider is the interface every LLM backend implements. The PRD requires
// this indirection so the church can switch providers without code changes.
type Provider interface {
	// Name identifies the provider and model for logging and audit.
	Name() string
	// Model returns the model identifier stored on the summary.
	Model() string
	// Summarize returns one summary per category group.
	Summarize(ctx context.Context, req Request) (*Response, error)
}

// Errors shared by providers and the service.
var (
	ErrTimeout     = errors.New("ai: provider timed out")
	ErrUnavailable = errors.New("ai: provider unavailable")
	ErrNoProvider  = errors.New("ai: no provider configured")
)

// NewProvider builds the provider named in configuration. When no provider or
// key is configured, a deterministic offline provider is returned so the app
// is fully usable without an LLM account (the summary is still a draft that a
// human must approve and edit).
func NewProvider(name, apiKey, model string, timeout time.Duration) Provider {
	switch name {
	case "openai":
		if apiKey != "" {
			return NewOpenAI(apiKey, model, timeout)
		}
	case "gemini":
		if apiKey != "" {
			return NewGemini(apiKey, model, timeout)
		}
	case "claude", "anthropic":
		if apiKey != "" {
			return NewClaude(apiKey, model, timeout)
		}
	}
	return NewOffline()
}

// wrapProviderError classifies transport failures into the documented codes.
func wrapProviderError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
