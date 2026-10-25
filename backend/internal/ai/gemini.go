package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Gemini calls the Google Generative Language API.
type Gemini struct {
	apiKey  string
	model   string
	client  *http.Client
	timeout time.Duration
}

// NewGemini builds the Google Gemini provider.
func NewGemini(apiKey, model string, timeout time.Duration) Provider {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &Gemini{
		apiKey:  apiKey,
		model:   model,
		client:  newHTTPClient(timeout),
		timeout: timeout,
	}
}

// Name identifies the provider.
func (g *Gemini) Name() string { return "gemini" }

// Model returns the configured model.
func (g *Gemini) Model() string { return g.model }

// Summarize requests category summaries from Gemini.
func (g *Gemini) Summarize(ctx context.Context, req Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	start := time.Now()
	url := "https://generativelanguage.googleapis.com/v1beta/models/" + g.model + ":generateContent"
	body := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": buildPrompt(req)}},
		},
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]string{{"text": buildUserPayload(req)}}},
		},
		"generationConfig": map[string]any{
			"temperature":      0.3,
			"responseMimeType": "application/json",
		},
	}

	raw, err := postJSON(ctx, g.client, url, map[string]string{"x-goog-api-key": g.apiKey}, body)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, wrapProviderError(ctx, err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, ErrUnavailable
	}

	shape, err := parseModelJSON(parsed.Candidates[0].Content.Parts[0].Text)
	if err != nil {
		return nil, err
	}
	return &Response{
		Summaries: attachSourceIDs(shape, req),
		ModelName: g.model,
		TokensIn:  parsed.UsageMetadata.PromptTokenCount,
		TokensOut: parsed.UsageMetadata.CandidatesTokenCount,
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}
