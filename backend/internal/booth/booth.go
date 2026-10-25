// Package booth implements prayer booth scheduling: shifts, assignments,
// rotation helper and swap requests (PRD F-09).
package booth

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/audit"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/conv"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
)

// Assignee is one intercessor on a shift.
type Assignee struct {
	AssignmentID string  `json:"assignment_id"`
	UserID       string  `json:"user_id"`
	FullName     string  `json:"full_name"`
	Status       string  `json:"status"`
	SwapWithID   *string `json:"swap_with_user_id"`
}

// Shift is one booth shift with its assignments.
type Shift struct {
	ID          string     `json:"id"`
	ServiceID   string     `json:"service_id"`
	Label       string     `json:"label"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	Capacity    int        `json:"capacity"`
	IsPublished bool       `json:"is_published"`
	Assignees   []Assignee `json:"assignees"`
	// Covered reports whether the shift has at least one assignee.
	Covered bool `json:"covered"`
	// Overbooked reports whether assignments exceed capacity.
	Overbooked bool `json:"overbooked"`
}

// SwapRequest is a pending or decided swap.
type SwapRequest struct {
	ID            string     `json:"id"`
	AssignmentID  string     `json:"assignment_id"`
	RequestedBy   string     `json:"requested_by"`
	RequesterName string     `json:"requester_name"`
	TargetUserID  string     `json:"target_user_id"`
	TargetName    string     `json:"target_name"`
	Status        string     `json:"status"`
	DecidedBy     *string    `json:"decided_by"`
	DecidedAt     *time.Time `json:"decided_at"`
	CreatedAt     time.Time  `json:"created_at"`
	// ShiftLabel and ShiftStartsAt make the list readable.
	ShiftLabel    string    `json:"shift_label"`
	ShiftStartsAt time.Time `json:"shift_starts_at"`
}

// Repository persists booth data.
type Repository struct {
	pool  *pgxpool.Pool
	audit *audit.Logger
}

// NewRepository builds the repository.
func NewRepository(pool *pgxpool.Pool, auditLog *audit.Logger) *Repository {
	return &Repository{pool: pool, audit: auditLog}
}

// ShiftsForService returns the roster for one service.
func (r *Repository) ShiftsForService(ctx context.Context, serviceID string) ([]Shift, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, service_id, label, starts_at, ends_at, capacity, is_published
		FROM booth_shifts
		WHERE service_id = $1 AND deleted_at IS NULL
		ORDER BY starts_at ASC
	`, serviceID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	shifts := []Shift{}
	index := map[string]int{}
	for rows.Next() {
		var s Shift
		if err := rows.Scan(&s.ID, &s.ServiceID, &s.Label, &s.StartsAt, &s.EndsAt, &s.Capacity, &s.IsPublished); err != nil {
			return nil, apperr.Internal(err)
		}
		s.Assignees = []Assignee{}
		index[s.ID] = len(shifts)
		shifts = append(shifts, s)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}

	if len(shifts) == 0 {
		return shifts, nil
	}

	ids := make([]string, 0, len(shifts))
	for _, s := range shifts {
		ids = append(ids, s.ID)
	}

	arows, err := r.pool.Query(ctx, `
		SELECT ba.id, ba.shift_id, ba.user_id, u.full_name, ba.status, ba.swap_with_user_id
		FROM booth_assignments ba
		JOIN users u ON u.id = ba.user_id
		WHERE ba.shift_id = ANY($1) AND ba.deleted_at IS NULL
		ORDER BY u.full_name
	`, ids)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer arows.Close()

	for arows.Next() {
		var (
			a      Assignee
			shiftID string
		)
		if err := arows.Scan(&a.AssignmentID, &shiftID, &a.UserID, &a.FullName, &a.Status, &a.SwapWithID); err != nil {
			return nil, apperr.Internal(err)
		}
		if i, ok := index[shiftID]; ok {
			shifts[i].Assignees = append(shifts[i].Assignees, a)
		}
	}
	if err := arows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}

	for i := range shifts {
		shifts[i].Covered = len(shifts[i].Assignees) > 0
		shifts[i].Overbooked = len(shifts[i].Assignees) > shifts[i].Capacity
	}
	return shifts, nil
}

// CreateShiftInput carries a new shift.
type CreateShiftInput struct {
	Label    string    `json:"label"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Capacity int       `json:"capacity"`
}

// CreateShift adds a shift to a service.
func (r *Repository) CreateShift(ctx context.Context, serviceID string, in CreateShiftInput, actorID string) (*Shift, error) {
	if in.Capacity < 1 || in.Capacity > 4 {
		return nil, apperr.Validation("Capacity must be between 1 and 4")
	}
	if !in.EndsAt.After(in.StartsAt) {
		return nil, apperr.Validation("Shift end time must be after the start time")
	}

	var s Shift
	err := r.pool.QueryRow(ctx, `
		INSERT INTO booth_shifts (service_id, label, starts_at, ends_at, capacity, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, service_id, label, starts_at, ends_at, capacity, is_published
	`, serviceID, in.Label, in.StartsAt, in.EndsAt, in.Capacity, actorID).
		Scan(&s.ID, &s.ServiceID, &s.Label, &s.StartsAt, &s.EndsAt, &s.Capacity, &s.IsPublished)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	s.Assignees = []Assignee{}

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "booth_shift.create", Entity: "booth_shifts", EntityID: s.ID,
	})
	return &s, nil
}

// Assign adds an intercessor to a shift, enforcing capacity.
func (r *Repository) Assign(ctx context.Context, shiftID, userID, actorID string) (*Assignee, error) {
	var capacity int
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT capacity, true FROM booth_shifts WHERE id = $1 AND deleted_at IS NULL
	`, shiftID).Scan(&capacity, &exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Shift")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}

	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM booth_assignments WHERE shift_id = $1 AND deleted_at IS NULL
	`, shiftID).Scan(&count); err != nil {
		return nil, apperr.Internal(err)
	}
	if count >= capacity {
		return nil, apperr.Unprocessable(apperr.CodeShiftOverbooked,
			"This shift is already at capacity. Increase capacity or pick another shift.")
	}

	var a Assignee
	err = r.pool.QueryRow(ctx, `
		INSERT INTO booth_assignments (shift_id, user_id, status, created_by, updated_by)
		VALUES ($1, $2, 'assigned', $3, $3)
		RETURNING id, user_id, status, swap_with_user_id
	`, shiftID, userID, actorID).Scan(&a.AssignmentID, &a.UserID, &a.Status, &a.SwapWithID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperr.Conflict("This intercessor is already assigned to the shift")
		}
		return nil, apperr.Internal(err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT full_name FROM users WHERE id = $1`, userID).Scan(&a.FullName); err != nil {
		a.FullName = ""
	}

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "booth_assignment.create", Entity: "booth_assignments", EntityID: a.AssignmentID,
	})
	return &a, nil
}

// Unassign soft-deletes an assignment.
func (r *Repository) Unassign(ctx context.Context, assignmentID, actorID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE booth_assignments SET deleted_at = now(), updated_by = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, assignmentID, actorID)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("Assignment")
	}
	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "booth_assignment.delete", Entity: "booth_assignments", EntityID: assignmentID,
	})
	return nil
}

// Publish marks every shift of a service as published.
func (r *Repository) Publish(ctx context.Context, serviceID, actorID string) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE booth_shifts SET is_published = true, updated_by = $2, updated_at = now()
		WHERE service_id = $1 AND deleted_at IS NULL
	`, serviceID, actorID)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "booth_roster.publish", Entity: "booth_shifts", Fields: []string{"is_published"},
	})
	return int(tag.RowsAffected()), nil
}

// RotationSuggestion returns the active intercessors ordered by fewest shifts
// in the last 4 weeks (PRD F-09 rotation helper, suggestion only).
func (r *Repository) RotationSuggestion(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.full_name, COUNT(ba.id) AS shifts_last_4_weeks
		FROM users u
		LEFT JOIN booth_assignments ba ON ba.user_id = u.id AND ba.deleted_at IS NULL
		LEFT JOIN booth_shifts bs ON bs.id = ba.shift_id AND bs.deleted_at IS NULL
			AND bs.starts_at > now() - interval '28 days'
		WHERE u.role = 'intercessor' AND u.is_active = true AND u.deleted_at IS NULL
		GROUP BY u.id, u.full_name
		ORDER BY shifts_last_4_weeks ASC, u.full_name ASC
	`)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var (
			id     string
			name   string
			count  int
		)
		if err := rows.Scan(&id, &name, &count); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, map[string]any{
			"user_id": id, "full_name": name, "shifts_last_4_weeks": count,
		})
	}
	return out, rows.Err()
}

// RequestSwap records a swap request and marks the assignment accordingly.
func (r *Repository) RequestSwap(ctx context.Context, assignmentID, requesterID, targetUserID string) (*SwapRequest, error) {
	var ownerID string
	var shiftID string
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, shift_id FROM booth_assignments WHERE id = $1 AND deleted_at IS NULL
	`, assignmentID).Scan(&ownerID, &shiftID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Assignment")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if ownerID != requesterID {
		return nil, apperr.Forbidden("You can only request a swap for your own shift")
	}
	if targetUserID == requesterID {
		return nil, apperr.Validation("Choose a different intercessor to swap with")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	var sr SwapRequest
	err = tx.QueryRow(ctx, `
		INSERT INTO swap_requests (assignment_id, requested_by, target_user_id)
		VALUES ($1, $2, $3)
		RETURNING id, assignment_id, requested_by, target_user_id, status, decided_by, decided_at, created_at
	`, assignmentID, requesterID, targetUserID).
		Scan(&sr.ID, &sr.AssignmentID, &sr.RequestedBy, &sr.TargetUserID, &sr.Status,
			&sr.DecidedBy, &sr.DecidedAt, &sr.CreatedAt)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE booth_assignments SET status = 'swap_requested', swap_with_user_id = $2, updated_at = now()
		WHERE id = $1
	`, assignmentID, targetUserID); err != nil {
		return nil, apperr.Internal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}

	r.notify(ctx, targetUserID, "Swap requested",
		"An intercessor asked to swap a prayer booth shift with you.", "/booth")
	return &sr, nil
}

// DecideSwap approves or declines a swap. Approval reassigns the shift.
func (r *Repository) DecideSwap(ctx context.Context, swapID, decision, actorID string) (*SwapRequest, error) {
	if decision != "approved" && decision != "declined" {
		return nil, apperr.Validation("Decision must be approved or declined")
	}

	var (
		sr         SwapRequest
		assignment string
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id, assignment_id, requested_by, target_user_id, status, created_at
		FROM swap_requests WHERE id = $1
	`, swapID).Scan(&sr.ID, &assignment, &sr.RequestedBy, &sr.TargetUserID, &sr.Status, &sr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Swap request")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if sr.Status != "pending" {
		return nil, apperr.Conflict("This swap request has already been decided")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE swap_requests SET status = $2, decided_by = $3, decided_at = now(), updated_at = now()
		WHERE id = $1
	`, swapID, decision, actorID); err != nil {
		return nil, apperr.Internal(err)
	}

	if decision == "approved" {
		if _, err := tx.Exec(ctx, `
			UPDATE booth_assignments
			SET user_id = $2, status = 'assigned', swap_with_user_id = NULL, updated_at = now()
			WHERE id = $1
		`, assignment, sr.TargetUserID); err != nil {
			return nil, apperr.Internal(err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE booth_assignments
			SET status = 'assigned', swap_with_user_id = NULL, updated_at = now()
			WHERE id = $1
		`, assignment); err != nil {
			return nil, apperr.Internal(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}

	msg := "Your swap request was declined."
	if decision == "approved" {
		msg = "Your swap request was approved. Check your updated roster."
	}
	r.notify(ctx, sr.RequestedBy, "Swap "+decision, msg, "/booth")
	r.notify(ctx, sr.TargetUserID, "Swap "+decision, msg, "/booth")

	r.audit.Record(ctx, audit.Entry{
		ActorID: actorID, Action: "swap_request." + decision, Entity: "swap_requests", EntityID: swapID,
	})
	return &sr, nil
}

// ListSwaps returns swap requests, filtered by status when given.
func (r *Repository) ListSwaps(ctx context.Context, status string) ([]SwapRequest, error) {
	q := `
		SELECT s.id, s.assignment_id, s.requested_by, ru.full_name, s.target_user_id, tu.full_name,
		       s.status, s.decided_by, s.decided_at, s.created_at, bs.label, bs.starts_at
		FROM swap_requests s
		JOIN users ru ON ru.id = s.requested_by
		JOIN users tu ON tu.id = s.target_user_id
		JOIN booth_assignments ba ON ba.id = s.assignment_id
		JOIN booth_shifts bs ON bs.id = ba.shift_id
		WHERE 1 = 1`
	args := []any{}
	if status != "" {
		args = append(args, status)
		q += ` AND s.status = $1`
	}
	q += ` ORDER BY s.created_at DESC LIMIT 100`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []SwapRequest{}
	for rows.Next() {
		var s SwapRequest
		if err := rows.Scan(&s.ID, &s.AssignmentID, &s.RequestedBy, &s.RequesterName,
			&s.TargetUserID, &s.TargetName, &s.Status, &s.DecidedBy, &s.DecidedAt,
			&s.CreatedAt, &s.ShiftLabel, &s.ShiftStartsAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// MyShifts returns the signed-in user's upcoming assignments.
func (r *Repository) MyShifts(ctx context.Context, userID string, limit int) ([]Shift, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT bs.id, bs.service_id, bs.label, bs.starts_at, bs.ends_at, bs.capacity, bs.is_published
		FROM booth_shifts bs
		JOIN booth_assignments ba ON ba.shift_id = bs.id
		WHERE ba.user_id = $1 AND ba.deleted_at IS NULL AND bs.deleted_at IS NULL
		  AND bs.ends_at > now()
		ORDER BY bs.starts_at ASC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()

	out := []Shift{}
	for rows.Next() {
		var s Shift
		if err := rows.Scan(&s.ID, &s.ServiceID, &s.Label, &s.StartsAt, &s.EndsAt, &s.Capacity, &s.IsPublished); err != nil {
			return nil, apperr.Internal(err)
		}
		s.Assignees = []Assignee{}
		s.Covered = true
		out = append(out, s)
	}
	return out, rows.Err()
}

// notify writes an in-app notification, ignoring failures.
func (r *Repository) notify(ctx context.Context, userID, title, body, link string) {
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, title, body, link) VALUES ($1, $2, $3, $4)
	`, userID, title, body, link)
}

func isUniqueViolation(err error) bool {
	type pgErr interface{ SQLState() string }
	var e pgErr
	if errors.As(err, &e) {
		return e.SQLState() == "23505"
	}
	return false
}

// Handler exposes booth endpoints.
type Handler struct{ repo *Repository }

// NewHandler builds the HTTP handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// Register mounts booth routes.
func (h *Handler) Register(g *gin.RouterGroup) {
	admin := auth.RequireRole(auth.RoleAdmin)
	member := auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor)

	g.GET("/services/:id/booth-shifts", member, h.listShifts)
	g.POST("/services/:id/booth-shifts", admin, h.createShift)
	g.POST("/services/:id/booth-roster/publish", admin, h.publish)
	g.POST("/booth-shifts/:id/assignments", admin, h.assign)
	g.DELETE("/booth-assignments/:id", admin, h.unassign)
	g.POST("/booth-assignments/:id/swap-requests", member, h.requestSwap)
	g.PATCH("/swap-requests/:id", admin, h.decideSwap)
	g.GET("/swap-requests", member, h.listSwaps)
	g.GET("/booth-shifts/mine", member, h.myShifts)
	g.GET("/booth-shifts/rotation", admin, h.rotation)
}

func (h *Handler) listShifts(c *gin.Context) {
	shifts, err := h.repo.ShiftsForService(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Roster retrieved", shifts)
}

type createShiftPayload struct {
	Label    string `json:"label" binding:"required"`
	StartsAt string `json:"starts_at" binding:"required"`
	EndsAt   string `json:"ends_at" binding:"required"`
	Capacity int    `json:"capacity"`
}

func (h *Handler) createShift(c *gin.Context) {
	var p createShiftPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("label, starts_at and ends_at are required"))
		return
	}
	starts, err := conv.ParseTime(p.StartsAt)
	if err != nil {
		response.Fail(c, apperr.Validation("starts_at must be an ISO 8601 datetime"))
		return
	}
	ends, err := conv.ParseTime(p.EndsAt)
	if err != nil {
		response.Fail(c, apperr.Validation("ends_at must be an ISO 8601 datetime"))
		return
	}
	if p.Capacity == 0 {
		p.Capacity = 2
	}
	s, err := h.repo.CreateShift(c.Request.Context(), c.Param("id"), CreateShiftInput{
		Label: p.Label, StartsAt: starts, EndsAt: ends, Capacity: p.Capacity,
	}, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Shift created", s)
}

type assignPayload struct {
	UserID string `json:"user_id" binding:"required"`
}

func (h *Handler) assign(c *gin.Context) {
	var p assignPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("user_id is required"))
		return
	}
	a, err := h.repo.Assign(c.Request.Context(), c.Param("id"), p.UserID, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Intercessor assigned", a)
}

func (h *Handler) unassign(c *gin.Context) {
	if err := h.repo.Unassign(c.Request.Context(), c.Param("id"), c.GetString(middleware.CtxUserID)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Assignment removed", gin.H{"id": c.Param("id"), "removed": true})
}

func (h *Handler) publish(c *gin.Context) {
	n, err := h.repo.Publish(c.Request.Context(), c.Param("id"), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Roster published", gin.H{"shifts_published": n})
}

type swapPayload struct {
	TargetUserID string `json:"target_user_id" binding:"required"`
}

func (h *Handler) requestSwap(c *gin.Context) {
	var p swapPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("target_user_id is required"))
		return
	}
	sr, err := h.repo.RequestSwap(c.Request.Context(), c.Param("id"), c.GetString(middleware.CtxUserID), p.TargetUserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, "Swap requested", sr)
}

type decidePayload struct {
	Status string `json:"status" binding:"required"`
}

func (h *Handler) decideSwap(c *gin.Context) {
	var p decidePayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, apperr.Validation("status is required"))
		return
	}
	sr, err := h.repo.DecideSwap(c.Request.Context(), c.Param("id"), p.Status, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Swap "+p.Status, sr)
}

func (h *Handler) listSwaps(c *gin.Context) {
	items, err := h.repo.ListSwaps(c.Request.Context(), c.Query("status"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Swap requests retrieved", items)
}

func (h *Handler) myShifts(c *gin.Context) {
	shifts, err := h.repo.MyShifts(c.Request.Context(), c.GetString(middleware.CtxUserID), 10)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Your shifts retrieved", shifts)
}

func (h *Handler) rotation(c *gin.Context) {
	items, err := h.repo.RotationSuggestion(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Rotation suggestion", items)
}
