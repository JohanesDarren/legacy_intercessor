package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Handler exposes the auth endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the auth HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic mounts the endpoints that need no access token.
func (h *Handler) RegisterPublic(g *gin.RouterGroup) {
	g.POST("/auth/login", h.login)
	g.POST("/auth/refresh", h.refresh)
	g.POST("/auth/accept-invite", h.acceptInvite)
	g.GET("/auth/invites/:token", h.previewInvite)
}

// RegisterProtected mounts the endpoints that require an access token.
func (h *Handler) RegisterProtected(g *gin.RouterGroup) {
	g.POST("/auth/logout", h.logout)
	g.POST("/auth/invites", RequireRole(RoleAdmin), h.createInvite)
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("Email and password are required"))
		return
	}
	pair, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Logged in successfully", pair)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *Handler) refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Unauthorized("Refresh token is required"))
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Token refreshed", pair)
}

func (h *Handler) logout(c *gin.Context) {
	var req refreshRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Logged out", gin.H{"revoked": true})
}

type createInviteRequest struct {
	Email    string `json:"email" binding:"required"`
	FullName string `json:"full_name"`
	Role     string `json:"role" binding:"required"`
}

func (h *Handler) createInvite(c *gin.Context) {
	var req createInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("Email and role are required"))
		return
	}
	actorID := c.GetString(middleware.CtxUserID)
	inv, err := h.svc.CreateInvite(c.Request.Context(), actorID, req.Email, req.FullName, req.Role)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Invite created", inv)
}

func (h *Handler) previewInvite(c *gin.Context) {
	preview, err := h.svc.PreviewInvite(c.Request.Context(), c.Param("token"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Invite is valid", preview)
}

type acceptInviteRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"full_name"`
}

func (h *Handler) acceptInvite(c *gin.Context) {
	var req acceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Validation("Token and password are required"))
		return
	}
	pair, err := h.svc.AcceptInvite(c.Request.Context(), req.Token, req.Password, req.FullName)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Account created", pair)
}
