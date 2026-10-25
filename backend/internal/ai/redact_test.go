package ai

import (
	"strings"
	"testing"
)

// T-01: records without display consent must never reach the provider.
func TestGroupByCategoryExcludesNonConsentedRecords(t *testing.T) {
	records := []SourceRecord{
		{ID: "a", CategoryID: "c1", CategoryName: "Health", Title: "One", Content: "x", DisplayConsent: ConsentYes},
		{ID: "b", CategoryID: "c1", CategoryName: "Health", Title: "Two", Content: "y", DisplayConsent: ConsentNo},
		{ID: "c", CategoryID: "c1", CategoryName: "Health", Title: "Three", Content: "z", DisplayConsent: ConsentAnonymous},
	}

	eligible := []SourceRecord{}
	for _, r := range records {
		if EligibleForDisplay(r.DisplayConsent) {
			eligible = append(eligible, r)
		}
	}

	groups := groupByCategory(eligible, "prayer")
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	for _, id := range groups[0].SourceIDs {
		if id == "b" {
			t.Fatal("T-01 FAILED: a record with consent=no reached the provider payload")
		}
	}
	if len(groups[0].Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(groups[0].Items))
	}
}

// T-02: anonymous records send a placeholder, never the real name.
func TestRedactNameAnonymousNeverLeaksName(t *testing.T) {
	if got := RedactName("Sari Wijaya", ConsentAnonymous); got != "Person A" {
		t.Fatalf("T-02 FAILED: expected 'Person A', got %q", got)
	}
	if got := RedactName("Sari Wijaya", ConsentYes); got != "Sari Wijaya" {
		t.Fatalf("consent=yes should keep the name, got %q", got)
	}
	if got := RedactName("Sari Wijaya", ConsentNo); got != "" {
		t.Fatalf("consent=no should drop the name, got %q", got)
	}
}

// T-06: follow-up notes are a separate type and can never be attached to a
// provider payload, because SourceRecord has no notes field at all.
func TestSourceRecordHasNoFollowUpField(t *testing.T) {
	r := SourceRecord{
		ID: "a", CategoryID: "c1", CategoryName: "Health",
		Title: "Title", Content: "Body", DisplayConsent: ConsentYes,
	}
	groups := groupByCategory([]SourceRecord{r}, "prayer")
	payload := buildUserPayload(Request{Kind: "prayer", Language: "en", Groups: groups})
	for _, forbidden := range []string{"notes", "follow_up", "channel", "condition_status"} {
		if strings.Contains(strings.ToLower(payload), forbidden) {
			t.Fatalf("T-06 FAILED: payload contains %q", forbidden)
		}
	}
}

func TestRedactTextRemovesContactDetails(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"indonesian phone", "Call me on 0812-3456-7890 please", "[phone removed]"},
		{"international phone", "Reach me at +62 812 3456 7890", "[phone removed]"},
		{"email", "Email sari@example.com for details", "[contact removed]"},
		{"street address", "She lives on Jl. Merdeka No 5", "[address removed]"},
		{"url", "See https://example.com/profile", "[link removed]"},
	}

	for _, tc := range cases {
		got := RedactText(tc.in)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: expected %q in %q", tc.name, tc.want, got)
		}
	}
}

// Ordinary numbers in prayer content must not be mangled.
func TestRedactTextKeepsOrdinaryNumbers(t *testing.T) {
	in := "She has 3 children and needs prayer for 2 weeks of recovery."
	got := RedactText(in)
	if strings.Contains(got, "removed") {
		t.Fatalf("ordinary numbers were redacted: %q", got)
	}
}

func TestEligibleForDisplay(t *testing.T) {
	if !EligibleForDisplay(ConsentYes) || !EligibleForDisplay(ConsentAnonymous) {
		t.Fatal("yes and anonymous must be eligible")
	}
	if EligibleForDisplay(ConsentNo) {
		t.Fatal("no must not be eligible")
	}
}

func TestParseModelJSONToleratesCodeFences(t *testing.T) {
	raw := "```json\n{\"summaries\":[{\"category\":\"Health\",\"text\":\"Pray for healing.\"}]}\n```"
	got, err := parseModelJSON(raw)
	if err != nil {
		t.Fatalf("expected to parse fenced JSON, got %v", err)
	}
	if len(got.Summaries) != 1 || got.Summaries[0].Category != "Health" {
		t.Fatalf("unexpected parse result: %+v", got.Summaries)
	}
}

func TestOfflineProviderDedupesTitles(t *testing.T) {
	p := NewOffline()
	req := Request{
		Kind: "praise", Language: "en", MaxWordsPerItem: 40,
		Groups: []CategoryGroup{{
			Category: "Healing", CategoryID: "c1", SourceIDs: []string{"a", "b", "c"},
			Items: []Item{
				{Title: "Healed after prayer"},
				{Title: "Healed after prayer"},
				{Title: "Healed after prayer"},
			},
		}},
	}
	resp, err := p.Summarize(t.Context(), req)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	text := resp.Summaries[0].Text
	if strings.Count(text, "healed after prayer") != 1 {
		t.Fatalf("duplicate titles should appear once, got %q", text)
	}
	if !strings.Contains(text, "3 testimony") {
		t.Fatalf("count should reflect all 3 records, got %q", text)
	}
}
