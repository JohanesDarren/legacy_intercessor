// Package response implements the single response envelope defined in PRD
// Section 6.4. No handler writes a JSON map by hand.
package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
)

// Envelope is the success shape for a single resource.
type Envelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ListEnvelope is the success shape for a paginated list.
type ListEnvelope struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Data    any      `json:"data"`
	Meta    PageMeta `json:"meta"`
}

// ErrorEnvelope is the shape returned for every failure.
type ErrorEnvelope struct {
	Status    string              `json:"status"`
	Message   string              `json:"message"`
	ErrorCode string              `json:"error_code"`
	Errors    map[string][]string `json:"errors,omitempty"`
}

// PageMeta carries pagination information.
type PageMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewPageMeta computes derived pagination fields.
func NewPageMeta(page, limit, total int) PageMeta {
	if limit < 1 {
		limit = 20
	}
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	return PageMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// OK writes a 200 response with the standard envelope.
func OK(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Envelope{Status: "success", Message: msg, Data: data})
}

// Created writes a 201 response with the standard envelope.
func Created(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusCreated, Envelope{Status: "success", Message: msg, Data: data})
}

// Accepted writes a 202 response, used for background export jobs.
func Accepted(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusAccepted, Envelope{Status: "success", Message: msg, Data: data})
}

// Paginated writes a 200 response with the list envelope and meta.
func Paginated(c *gin.Context, msg string, data any, meta PageMeta) {
	c.JSON(http.StatusOK, ListEnvelope{Status: "success", Message: msg, Data: data, Meta: meta})
}

// NoContent writes a 204 response with no body.
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// Fail maps an error to the error envelope and the right HTTP status.
func Fail(c *gin.Context, err error) {
	ae := apperr.From(err)

	// Log the internal cause so operators can diagnose a 500, while the client
	// still only sees the safe message. Never log request content.
	if ae.Status >= 500 && ae.Err != nil {
		slog.Error("request failed",
			slog.String("request_id", c.GetString("request_id")),
			slog.String("route", c.FullPath()),
			slog.String("error_code", ae.Code),
			slog.String("cause", ae.Err.Error()),
		)
	}

	c.AbortWithStatusJSON(ae.Status, ErrorEnvelope{
		Status:    "error",
		Message:   ae.Message,
		ErrorCode: ae.Code,
		Errors:    ae.Fields,
	})
}
