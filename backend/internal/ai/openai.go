package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// OpenAI calls the Chat Completions API.
type OpenAI struct {
	apiKey  string
	model   string
	client  *http.Client
	timeout time.Duration
}

// NewOpenAI builds the OpenAI provider.
func NewOpenAI(apiKey, model string, timeout time.Duration) Provider {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAI{
		apiKey:  apiKey,
		model:   model,
		client:  newHTTPClient(timeout),
		timeout: timeout,
	}
}

// Name identifies the provider.
func (o *OpenAI) Name() string { return "openai" }

// Model returns the configured model.
func (o *OpenAI) Model() string { return o.model }

// Summarize requests category summaries from OpenAI.
func (o *OpenAI) Summarize(ctx context.Context, req Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	start := time.Now()
	body := map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": buildPrompt(req)},
			{"role": "user", "content": buildUserPayload(req)},
		},
		"temperature":     0.3,
		"response_format": map[string]string{"type": "json_object"},
	}

	raw, err := postJSON(ctx, o.client, "https://api.openai.com/v1/chat/completions", map[string]string{
		"Authorization": "Bearer " + o.apiKey,
	}, body)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, wrapProviderError(ctx, err)
	}
	if len(parsed.Choices) == 0 {
		return nil, ErrUnavailable
	}

	shape, err := parseModelJSON(parsed.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	return &Response{
		Summaries: attachSourceIDs(shape, req),
		ModelName: o.model,
		TokensIn:  parsed.Usage.PromptTokens,
		TokensOut: parsed.Usage.CompletionTokens,
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}
