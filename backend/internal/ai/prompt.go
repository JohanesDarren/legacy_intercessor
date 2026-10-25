package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// promptTemplate is the shared instruction block. It is versioned with
// PromptVersion so the stored summary records which wording produced it.
const promptTemplate = `You are helping the intercessor team at Legacy Sukawarna church prepare
short, gentle summaries of prayer requests or praise reports for a videotron slide.

Rules you must follow:
1. Write one summary per category, in the requested language.
2. Be pastoral, warm and concise. Do not add facts that are not in the input.
3. Never invent names, diagnoses, or numbers.
4. Do not mention contact details, addresses, phone numbers, or how the team followed up.
5. Keep each summary to at most %d words per item.
6. Return ONLY valid JSON in this exact shape:
{"summaries":[{"category":"<category name>","text":"<summary>"}]}
7. Include every category that appears in the input, once.`

// buildPrompt renders the instruction block for a request.
func buildPrompt(req Request) string {
	return fmt.Sprintf(promptTemplate, req.MaxWordsPerItem)
}

// buildUserPayload renders the redacted input as compact JSON.
func buildUserPayload(req Request) string {
	type payloadItem struct {
		Name    string `json:"name,omitempty"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	type payloadGroup struct {
		Category string        `json:"category"`
		Items    []payloadItem `json:"items"`
	}
	out := struct {
		Kind     string         `json:"kind"`
		Language string         `json:"language"`
		Groups   []payloadGroup `json:"groups"`
	}{Kind: req.Kind, Language: req.Language}

	for _, g := range req.Groups {
		pg := payloadGroup{Category: g.Category}
		for _, it := range g.Items {
			pg.Items = append(pg.Items, payloadItem{Name: it.DisplayName, Title: it.Title, Content: it.Content})
		}
		out.Groups = append(out.Groups, pg)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// summaryShape is the JSON the model is asked to return.
type summaryShape struct {
	Summaries []struct {
		Category string `json:"category"`
		Text     string `json:"text"`
	} `json:"summaries"`
}

// parseModelJSON tolerates code fences and leading prose around the JSON body.
func parseModelJSON(raw string) (*summaryShape, error) {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 && j < len(s)-1 {
		s = s[:j+1]
	}
	var out summaryShape
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("ai: model returned unparseable JSON: %w", err)
	}
	if len(out.Summaries) == 0 {
		return nil, fmt.Errorf("ai: model returned no summaries")
	}
	return &out, nil
}

// attachSourceIDs maps model categories back to the groups we sent, so every
// draft carries the record IDs it came from (PRD F-06).
func attachSourceIDs(parsed *summaryShape, req Request) []CategorySummary {
	out := make([]CategorySummary, 0, len(parsed.Summaries))
	for _, s := range parsed.Summaries {
		cs := CategorySummary{Category: s.Category, Text: strings.TrimSpace(s.Text)}
		for _, g := range req.Groups {
			if strings.EqualFold(g.Category, s.Category) {
				cs.CategoryID = g.CategoryID
				cs.SourceIDs = g.SourceIDs
				break
			}
		}
		out = append(out, cs)
	}
	return out
}

// postJSON performs a JSON POST and returns the response body.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, wrapProviderError(ctx, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, wrapProviderError(ctx, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return data, nil
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
