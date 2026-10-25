package presentation

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Handler exposes presentation and export endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds the HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register mounts presentation routes. Viewers may read presentations and
// exports but not create or export (PRD 6.4).
func (h *Handler) Register(g *gin.RouterGroup) {
	all := auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor, auth.RoleViewer)
	admin := auth.RequireRole(auth.RoleAdmin)

	g.GET("/presentations", all, h.list)
	g.GET("/presentations/:id", all, h.get)
	g.POST("/presentations", admin, h.create)
	g.PATCH("/presentations/:id", admin, h.update)
	g.POST("/presentations/:id/exports", admin, h.startExport)
	g.GET("/presentations/:id/exports", all, h.listExports)
	g.GET("/presentations/:id/exports/:exportId", all, h.getExport)
	g.GET("/exports/latest", all, h.latestExport)
}

func (h *Handler) list(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), c.Query("service_id"), c.Query("kind"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Presentations retrieved", items)
}

func (h *Handler) get(c *gin.Context) {
	p, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	// Viewers only see presentations that have been exported (approved output).
	if c.GetString(middleware.CtxUserRole) == auth.RoleViewer && p.Status != "exported" {
		response.Fail(c, apperr.Forbidden("Only exported presentations are visible to viewers"))
		return
	}
	response.OK(c, "Presentation retrieved", p)
}

type buildPayload struct {
	ServiceID string `json:"service_id" binding:"required"`
	Kind      string `json:"kind" binding:"required"`
	Note      string `json:"note"`
}

func (h *Handler) create(c *gin.Context) {
	var p buildPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("service_id and kind are required"))
		return
	}
	if p.Kind != "prayer" && p.Kind != "praise" {
		response.Fail(c, apperr.Validation("kind must be prayer or praise"))
		return
	}
	out, err := h.svc.Build(c.Request.Context(), BuildInput(p), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Presentation created", out)
}

type updatePayload struct {
	Slides *[]Slide `json:"slides"`
	Note   *string  `json:"note"`
}

func (h *Handler) update(c *gin.Context) {
	var p updatePayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("Invalid JSON body"))
		return
	}
	out, err := h.svc.Update(c.Request.Context(), c.Param("id"), UpdateInput(p), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Presentation updated", out)
}

type exportPayload struct {
	Format string `json:"format" binding:"required"`
}

func (h *Handler) startExport(c *gin.Context) {
	var p exportPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("format is required"))
		return
	}
	e, err := h.svc.StartExport(c.Request.Context(), c.Param("id"), p.Format, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Accepted(c, "Export job created", e)
}

func (h *Handler) listExports(c *gin.Context) {
	items, err := h.svc.ListExports(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Exports retrieved", items)
}

func (h *Handler) getExport(c *gin.Context) {
	e, err := h.svc.GetExport(c.Request.Context(), c.Param("exportId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Export retrieved", e)
}

func (h *Handler) latestExport(c *gin.Context) {
	serviceID := c.Query("service_id")
	kind := c.DefaultQuery("kind", "prayer")
	if serviceID == "" {
		response.Fail(c, apperr.Validation("service_id is required"))
		return
	}
	e, err := h.svc.LatestApprovedExport(c.Request.Context(), serviceID, kind)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Latest export retrieved", e)
}

// FileHandler serves locally stored exports (the local storage driver).
type FileHandler struct {
	root string
}

// NewFileHandler builds the download handler for the local driver.
func NewFileHandler(root string) *FileHandler { return &FileHandler{root: root} }

// Register mounts the download route. Keys are validated to stay inside root.
func (h *FileHandler) Register(g *gin.RouterGroup) {
	g.GET("/files/*key", auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor, auth.RoleViewer), h.download)
}

func (h *FileHandler) download(c *gin.Context) {
	key := c.Param("key")
	if key == "" || key == "/" {
		response.Fail(c, apperr.NotFound("File"))
		return
	}
	clean := filepath.Clean("/" + key)
	full := filepath.Join(h.root, clean)
	if !withinRoot(h.root, full) {
		response.Fail(c, apperr.Forbidden("Invalid file key"))
		return
	}

	f, err := openFile(full)
	if err != nil {
		response.Fail(c, apperr.NotFound("File"))
		return
	}
	defer f.Close()

	c.Header("Content-Disposition", `attachment; filename="`+filepath.Base(full)+`"`)
	c.Header("Cache-Control", "private, max-age=600")
	c.Header("X-Content-Type-Options", "nosniff")
	if filepath.Ext(full) == ".pdf" {
		c.Header("Content-Type", "application/pdf")
	} else {
		c.Header("Content-Type", "application/zip")
	}
	if _, err := io.Copy(c.Writer, f); err != nil {
		_ = c.Error(err)
	}
}

func withinRoot(root, full string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return false
	}
	return rel != ".." && !hasDotDotPrefix(rel)
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && (rel[2] == '/' || rel[2] == '\\')
}

// openFile opens a file for reading, reporting a clear error when missing.
func openFile(path string) (*os.File, error) {
	return os.Open(path)
}
