// Package presentation builds slide sets and exports them to PNG and PDF using
// headless Chromium (PRD F-07). Preview and export share one template source so
// the output always matches what the admin sees.
package presentation

import (
	"time"
)

// SlideItem is one record placed on a slide, with any text override applied.
type SlideItem struct {
	// RefID is the prayer request or praise report ID.
	RefID string `json:"ref_id"`
	// Kind is "prayer" or "praise".
	Kind string `json:"kind"`
	// Title and Body are the rendered text, possibly overridden by the admin.
	Title string `json:"title"`
	Body  string `json:"body"`
	// Name is empty for anonymous or unnamed records.
	Name string `json:"name,omitempty"`
	// IsUrgent drives the orange URGENT marker (prayer only).
	IsUrgent bool `json:"is_urgent,omitempty"`
	// SourceSummaryID is set when the text came from an approved AI summary.
	SourceSummaryID string `json:"source_summary_id,omitempty"`
}

// Slide is one 1920x1080 canvas.
type Slide struct {
	ID       string      `json:"id"`
	Template string      `json:"template"` // "prayer" | "praise"
	// Title is the slide heading, usually the category name.
	Title string      `json:"title"`
	Items []SlideItem `json:"items"`
	// Note is an optional subtitle such as the service date.
	Note string `json:"note,omitempty"`
}

// Presentation is a versioned slide set for one service and kind.
type Presentation struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Kind      string    `json:"kind"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	Slides    []Slide   `json:"slides"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `json:"created_by"`
}

// Export is an export job.
type Export struct {
	ID             string     `json:"id"`
	PresentationID string     `json:"presentation_id"`
	Format         string     `json:"format"`
	Status         string     `json:"status"`
	StorageKey     *string    `json:"storage_key"`
	DownloadURL    *string    `json:"download_url"`
	ErrorMessage   *string    `json:"error_message"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

// TemplateConfig holds the pagination limits from PRD F-07.
type TemplateConfig struct {
	// ItemsPerSlide is 4 for prayer and 3 for praise.
	ItemsPerSlide int
	// Title is the default slide heading.
	Title string
}

// ConfigFor returns the template rules for a kind.
func ConfigFor(kind string) TemplateConfig {
	if kind == "praise" {
		return TemplateConfig{ItemsPerSlide: 3, Title: "Praise Reports"}
	}
	return TemplateConfig{ItemsPerSlide: 4, Title: "Prayer Requests"}
}
