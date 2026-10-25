package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Claude calls the Anthropic Messages API.
type Claude struct {
	apiKey  string
	model   string
	client  *http.Client
	timeout time.Duration
}

// NewClaude builds the Anthropic provider.
func NewClaude(apiKey, model string, timeout time.Duration) Provider {
	if model == "" {
		model = "claude-3-5-haiku-latest"
	}
	return &Claude{
		apiKey:  apiKey,
		model:   model,
		client:  newHTTPClient(timeout),
		timeout: timeout,
	}
}

// Name identifies the provider.
func (c *Claude) Name() string { return "claude" }

// Model returns the configured model.
func (c *Claude) Model() string { return c.model }

// Summarize requests category summaries from Anthropic.
func (c *Claude) Summarize(ctx context.Context, req Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	body := map[string]any{
		"model":      c.model,
		"max_tokens": 2000,
		"system":     buildPrompt(req),
		"messages": []map[string]string{
			{"role": "user", "content": buildUserPayload(req)},
		},
	}

	raw, err := postJSON(ctx, c.client, "https://api.anthropic.com/v1/messages", map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, wrapProviderError(ctx, err)
	}

	var text string
	for _, blk := range parsed.Content {
		if blk.Type == "text" {
			text += blk.Text
		}
	}
	if text == "" {
		return nil, ErrUnavailable
	}

	shape, err := parseModelJSON(text)
	if err != nil {
		return nil, err
	}
	return &Response{
		Summaries: attachSourceIDs(shape, req),
		ModelName: c.model,
		TokensIn:  parsed.Usage.InputTokens,
		TokensOut: parsed.Usage.OutputTokens,
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}
