package ai

import (
	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Handler exposes the summary endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the AI HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register mounts AI routes. Only admins can generate or approve (PRD 6.4).
func (h *Handler) Register(g *gin.RouterGroup, loadRecords func(ctx *gin.Context, serviceID, kind string) ([]SourceRecord, error)) {
	admin := auth.RequireRole(auth.RoleAdmin)
	g.POST("/ai/summaries", admin, h.generate(loadRecords))
	g.GET("/ai/summaries", admin, h.list)
	g.PATCH("/ai/summaries/:id", admin, h.review)
	g.GET("/ai/status", auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor), h.status)
}

func (h *Handler) generate(load func(ctx *gin.Context, serviceID, kind string) ([]SourceRecord, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in GenerateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			response.Fail(c, apperr.Validation("service_id and kind are required"))
			return
		}
		if in.ServiceID == "" {
			response.Fail(c, apperr.Validation("service_id is required"))
			return
		}
		if in.Kind != "prayer" && in.Kind != "praise" {
			response.Fail(c, apperr.Validation("kind must be prayer or praise"))
			return
		}
		if in.Language == "" {
			in.Language = "id"
		}
		if in.Language != "id" && in.Language != "en" {
			response.Fail(c, apperr.Validation("language must be id or en"))
			return
		}
		if in.MaxWordsPerItem <= 0 {
			in.MaxWordsPerItem = 40
		}
		if in.MaxWordsPerItem > 120 {
			in.MaxWordsPerItem = 120
		}

		records, err := load(c, in.ServiceID, in.Kind)
		if err != nil {
			response.Fail(c, err)
			return
		}

		out, err := h.svc.Generate(c.Request.Context(), in, records, c.GetString(middleware.CtxUserID))
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.Created(c, "Draft summaries generated", out)
	}
}

func (h *Handler) list(c *gin.Context) {
	serviceID := c.Query("service_id")
	if serviceID == "" {
		response.Fail(c, apperr.Validation("service_id is required"))
		return
	}
	items, err := h.svc.ListByService(c.Request.Context(), serviceID, c.Query("kind"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Summaries retrieved", items)
}

type reviewPayload struct {
	FinalText *string `json:"final_text"`
	Status    *string `json:"status"`
}

func (h *Handler) review(c *gin.Context) {
	var p reviewPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}
	sm, err := h.svc.Review(c.Request.Context(), c.Param("id"), ReviewInput(p), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Summary updated", sm)
}

func (h *Handler) status(c *gin.Context) {
	response.OK(c, "AI provider status", gin.H{
		"provider":       h.svc.ProviderName(),
		"model":          h.svc.ProviderModel(),
		"prompt_version": PromptVersion,
		"offline_mode":   h.svc.ProviderName() == "offline",
	})
}
