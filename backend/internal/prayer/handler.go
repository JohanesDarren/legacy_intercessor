package prayer

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/conv"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Handler exposes the prayer request endpoints (PRD 6.4).
type Handler struct {
	repo *Repository
}

// NewHandler builds the prayer HTTP handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register mounts prayer routes. Viewers are excluded by role, which is the
// F-01 acceptance criterion "viewer requests a prayer list -> 403".
func (h *Handler) Register(g *gin.RouterGroup) {
	writers := auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor)
	g.GET("/prayer-requests", writers, h.list)
	g.GET("/prayer-requests/:id", writers, h.get)
	g.POST("/prayer-requests", writers, h.create)
	g.PUT("/prayer-requests/:id", writers, h.update)
	g.PATCH("/prayer-requests/:id", writers, h.update)
	g.DELETE("/prayer-requests/:id", auth.RequireRole(auth.RoleAdmin), h.remove)
}

func (h *Handler) list(c *gin.Context) {
	f := Filter{
		Page:       intQuery(c, "page", 1),
		Limit:      intQuery(c, "limit", 20),
		ServiceID:  c.Query("service_id"),
		CategoryID: c.Query("category_id"),
		Gender:     c.Query("gender"),
		Status:     c.Query("status"),
		Consent:    c.Query("consent"),
		Search:     c.Query("q"),
		SortBy:     c.DefaultQuery("sort_by", "received_at"),
		SortOrder:  c.DefaultQuery("sort_order", "desc"),
		CreatedBy:  c.Query("created_by"),
	}
	if v := c.Query("urgent"); v != "" {
		b := v == "true" || v == "1"
		f.Urgent = &b
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Page < 1 {
		f.Page = 1
	}

	items, total, err := h.repo.List(c.Request.Context(), f)
	if err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.Paginated(c, "Prayer requests retrieved", items, response.NewPageMeta(f.Page, f.Limit, total))
}

func (h *Handler) get(c *gin.Context) {
	req, err := h.repo.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.OK(c, "Prayer request retrieved", req)
}

type createPayload struct {
	ServiceID        *string    `json:"service_id"`
	CategoryID       string     `json:"category_id"`
	RequesterName    *string    `json:"requester_name"`
	RequesterGender  string     `json:"requester_gender"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	IsUrgent         bool       `json:"is_urgent"`
	Status           string     `json:"status"`
	DisplayConsent   string     `json:"display_consent"`
	ReceivedAt       *string    `json:"received_at"`
	ConfirmDuplicate bool       `json:"confirm_duplicate"`
}

func (h *Handler) create(c *gin.Context) {
	var p createPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}

	if fields := validate(p.Title, p.Content, p.CategoryID, p.RequesterGender, p.Status, p.DisplayConsent); len(fields) > 0 {
		response.Fail(c, apperr.ValidationFields("Please correct the highlighted fields", fields))
		return
	}

	in := CreateInput{
		ServiceID:        p.ServiceID,
		CategoryID:       p.CategoryID,
		RequesterName:    p.RequesterName,
		RequesterGender:  p.RequesterGender,
		Title:            strings.TrimSpace(p.Title),
		Content:          strings.TrimSpace(p.Content),
		IsUrgent:         p.IsUrgent,
		DisplayConsent:   p.DisplayConsent,
		Status:           p.Status,
		ConfirmDuplicate: p.ConfirmDuplicate,
	}
	if p.ReceivedAt != nil && *p.ReceivedAt != "" {
		t, err := conv.ParseTime(*p.ReceivedAt)
		if err != nil {
			response.Fail(c, apperr.Validation("received_at must be an ISO 8601 datetime"))
			return
		}
		in.ReceivedAt = &t
	}

	req, err := h.repo.Create(c.Request.Context(), in, c.GetString(middleware.CtxUserID), c.ClientIP())
	if err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.Created(c, "Prayer request created", req)
}

type updatePayload struct {
	ServiceID       *string `json:"service_id"`
	CategoryID      *string `json:"category_id"`
	RequesterName   *string `json:"requester_name"`
	RequesterGender *string `json:"requester_gender"`
	Title           *string `json:"title"`
	Content         *string `json:"content"`
	IsUrgent        *bool   `json:"is_urgent"`
	Status          *string `json:"status"`
	DisplayConsent  *string `json:"display_consent"`
	ReceivedAt      *string `json:"received_at"`
}

func (h *Handler) update(c *gin.Context) {
	var p updatePayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}

	fields := map[string][]string{}
	if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
		fields["title"] = []string{"Title cannot be empty"}
	}
	if p.Title != nil && len(*p.Title) > 255 {
		fields["title"] = []string{"Title must be at most 255 characters"}
	}
	if p.Content != nil && strings.TrimSpace(*p.Content) == "" {
		fields["content"] = []string{"Content cannot be empty"}
	}
	if p.Content != nil && len(*p.Content) > 5000 {
		fields["content"] = []string{"Content must be at most 5000 characters"}
	}
	if p.Status != nil && !validStatus(*p.Status) {
		fields["status"] = []string{"Status must be open, in_prayer, answered or closed"}
	}
	if p.DisplayConsent != nil && !validConsent(*p.DisplayConsent) {
		fields["display_consent"] = []string{"Display consent must be yes, anonymous or no"}
	}
	if p.RequesterGender != nil && !validGender(*p.RequesterGender) {
		fields["requester_gender"] = []string{"Gender must be male, female or unspecified"}
	}
	if len(fields) > 0 {
		response.Fail(c, apperr.ValidationFields("Please correct the highlighted fields", fields))
		return
	}

	in := UpdateInput{
		ServiceID:       p.ServiceID,
		CategoryID:      p.CategoryID,
		RequesterName:   p.RequesterName,
		RequesterGender: p.RequesterGender,
		Title:           p.Title,
		Content:         p.Content,
		IsUrgent:        p.IsUrgent,
		Status:          p.Status,
		DisplayConsent:  p.DisplayConsent,
	}
	if p.ReceivedAt != nil && *p.ReceivedAt != "" {
		t, err := conv.ParseTime(*p.ReceivedAt)
		if err != nil {
			response.Fail(c, apperr.Validation("received_at must be an ISO 8601 datetime"))
			return
		}
		in.ReceivedAt = &t
	}

	req, err := h.repo.Update(c.Request.Context(), c.Param("id"), in, c.GetString(middleware.CtxUserID), c.ClientIP())
	if err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.OK(c, "Prayer request updated", req)
}

func (h *Handler) remove(c *gin.Context) {
	if err := h.repo.SoftDelete(c.Request.Context(), c.Param("id"), c.GetString(middleware.CtxUserID), c.ClientIP()); err != nil {
		response.Fail(c, mapErr(err))
		return
	}
	response.OK(c, "Prayer request deleted", gin.H{"id": c.Param("id"), "deleted": true})
}

// mapErr converts repository errors into the documented API errors.
func mapErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return apperr.NotFound("Prayer request")
	case errors.Is(err, ErrDuplicate):
		return apperr.DuplicateRequest("A similar request from this person was saved in the last 7 days. Resend with confirm_duplicate: true to save anyway.")
	default:
		return apperr.Internal(err)
	}
}

func validate(title, content, categoryID, gender, status, consent string) map[string][]string {
	f := map[string][]string{}
	if strings.TrimSpace(title) == "" {
		f["title"] = []string{"Title is required"}
	} else if len(title) > 255 {
		f["title"] = []string{"Title must be at most 255 characters"}
	}
	if strings.TrimSpace(content) == "" {
		f["content"] = []string{"Content is required"}
	} else if len(content) > 5000 {
		f["content"] = []string{"Content must be at most 5000 characters"}
	}
	if strings.TrimSpace(categoryID) == "" {
		f["category_id"] = []string{"Category is required"}
	}
	if gender != "" && !validGender(gender) {
		f["requester_gender"] = []string{"Gender must be male, female or unspecified"}
	}
	if status != "" && !validStatus(status) {
		f["status"] = []string{"Status must be open, in_prayer, answered or closed"}
	}
	if consent != "" && !validConsent(consent) {
		f["display_consent"] = []string{"Display consent must be yes, anonymous or no"}
	}
	return f
}

func validGender(g string) bool {
	return g == "male" || g == "female" || g == "unspecified"
}

func validStatus(s string) bool {
	return s == "open" || s == "in_prayer" || s == "answered" || s == "closed"
}

func validConsent(c string) bool {
	return c == "yes" || c == "anonymous" || c == "no"
}

func intQuery(c *gin.Context, key string, fallback int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
