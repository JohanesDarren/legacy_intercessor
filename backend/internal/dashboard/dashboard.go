// Package dashboard assembles the home screen payload (PRD F-02): the next
// service, the slide deadline countdown, urgent requests needing attention,
// this week's record counts, and the signed-in user's next booth shift.
package dashboard

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/auth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/booth"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/apperr"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/conv"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/middleware"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/platform/response"
	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/services"
)

// NextService is the service card.
type NextService struct {
	ID            string     `json:"id"`
	StartsAt      time.Time  `json:"starts_at"`
	Location      string     `json:"location"`
	SlideDeadline *time.Time `json:"slide_deadline"`
	// DeadlineHoursLeft is negative once the deadline has passed.
	DeadlineHoursLeft float64 `json:"deadline_hours_left"`
	// DeadlinePassed is true when the deadline is in the past.
	DeadlinePassed bool `json:"deadline_passed"`
}

// UrgentItem is one urgent request needing attention.
type UrgentItem struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	CategoryName   string     `json:"category_name"`
	RequesterName  *string    `json:"requester_name"`
	ReceivedAt     time.Time  `json:"received_at"`
	LastFollowUpAt *time.Time `json:"last_follow_up_at"`
	FollowUpCount  int        `json:"follow_up_count"`
	// DaysSinceFollowUp is 999 when there has never been a follow-up.
	DaysSinceFollowUp int `json:"days_since_follow_up"`
}

// DeadlineStatus summarises whether the slides are ready.
type DeadlineStatus struct {
	// PresentationExists is true when a presentation was built for the service.
	PresentationExists bool `json:"presentation_exists"`
	// Exported is true when at least one export completed.
	Exported bool `json:"exported"`
	// Warning is the banner text when action is needed.
	Warning string `json:"warning,omitempty"`
}

// Summary is the full dashboard payload.
type Summary struct {
	NextService     *NextService   `json:"next_service"`
	Deadline        DeadlineStatus `json:"deadline_status"`
	UrgentCount     int            `json:"urgent_count"`
	UrgentRequests  []UrgentItem   `json:"urgent_requests"`
	PrayerThisWeek  int            `json:"prayer_this_week"`
	PraiseThisWeek  int            `json:"praise_this_week"`
	MyNextShift     *booth.Shift   `json:"my_next_shift"`
	DraftSummaryCount int          `json:"draft_summary_count"`
	UnreadNotifications int        `json:"unread_notifications"`
	ServerTime      time.Time      `json:"server_time"`
	Timezone        string         `json:"timezone"`
}

// Service builds the dashboard.
type Service struct {
	pool     *pgxpool.Pool
	services *services.Repository
	booth    *booth.Repository
}

// NewService wires the dashboard service.
func NewService(pool *pgxpool.Pool, svcRepo *services.Repository, boothRepo *booth.Repository) *Service {
	return &Service{pool: pool, services: svcRepo, booth: boothRepo}
}

// Build assembles the dashboard for one user.
func (s *Service) Build(ctx context.Context, userID string) (*Summary, error) {
	out := &Summary{
		UrgentRequests: []UrgentItem{},
		ServerTime:     time.Now(),
		Timezone:       "Asia/Jakarta",
	}

	next, err := s.services.Next(ctx)
	if err == nil {
		loc := conv.Jakarta()
		ns := &NextService{
			ID: next.ID, StartsAt: next.StartsAt.In(loc),
			Location: next.Location, SlideDeadline: next.SlideDeadline,
		}
		if next.SlideDeadline != nil {
			left := time.Until(*next.SlideDeadline).Hours()
			ns.DeadlineHoursLeft = round1(left)
			ns.DeadlinePassed = left < 0
		}
		out.NextService = ns
	}

	// Urgent requests for the next service that have no follow-up in 7 days.
	if out.NextService != nil {
		out.UrgentRequests, out.UrgentCount, err = s.urgentRequests(ctx, out.NextService.ID)
		if err != nil {
			return nil, err
		}
		out.Deadline, err = s.deadlineStatus(ctx, out.NextService)
		if err != nil {
			return nil, err
		}
	}

	if err := s.weekCounts(ctx, out); err != nil {
		return nil, err
	}

	shifts, err := s.booth.MyShifts(ctx, userID, 1)
	if err == nil && len(shifts) > 0 {
		out.MyNextShift = &shifts[0]
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL
	`, userID).Scan(&out.UnreadNotifications); err != nil {
		out.UnreadNotifications = 0
	}

	return out, nil
}

// urgentRequests returns urgent, open requests ordered by staleness. Requests
// with a follow-up in the last 7 days are excluded from the reminder list.
func (s *Service) urgentRequests(ctx context.Context, serviceID string) ([]UrgentItem, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.title, c.name, p.requester_name, p.received_at,
		       f.last_follow_up_at, COALESCE(f.cnt, 0)
		FROM prayer_requests p
		JOIN categories c ON c.id = p.category_id
		LEFT JOIN (
			SELECT prayer_request_id, MAX(created_at) AS last_follow_up_at, COUNT(*) AS cnt
			FROM follow_ups WHERE deleted_at IS NULL GROUP BY prayer_request_id
		) f ON f.prayer_request_id = p.id
		WHERE p.deleted_at IS NULL AND p.is_urgent = true
		  AND p.status IN ('open', 'in_prayer')
		  AND ($1::uuid IS NULL OR p.service_id = $1::uuid)
		  AND (f.last_follow_up_at IS NULL OR f.last_follow_up_at < now() - interval '7 days')
		ORDER BY f.last_follow_up_at NULLS FIRST, p.received_at ASC
		LIMIT 20
	`, nullableUUID(serviceID))
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	defer rows.Close()

	items := []UrgentItem{}
	for rows.Next() {
		var it UrgentItem
		if err := rows.Scan(&it.ID, &it.Title, &it.CategoryName, &it.RequesterName,
			&it.ReceivedAt, &it.LastFollowUpAt, &it.FollowUpCount); err != nil {
			return nil, 0, apperr.Internal(err)
		}
		if it.LastFollowUpAt == nil {
			it.DaysSinceFollowUp = 999
		} else {
			it.DaysSinceFollowUp = int(time.Since(*it.LastFollowUpAt).Hours() / 24)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM prayer_requests
		WHERE deleted_at IS NULL AND is_urgent = true
		  AND status IN ('open', 'in_prayer')
		  AND ($1::uuid IS NULL OR service_id = $1::uuid)
	`, nullableUUID(serviceID)).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return items, total, nil
}

// nullableUUID converts an empty string to a NULL uuid so the query can use a
// single optional filter without a uuid/text type clash.
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// deadlineStatus reports whether slides exist and were exported, and builds
// the warning banner required by the F-02 acceptance criteria.
func (s *Service) deadlineStatus(ctx context.Context, ns *NextService) (DeadlineStatus, error) {
	var status DeadlineStatus

	var presentationID *string
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM presentations
		WHERE service_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1
	`, ns.ID).Scan(&presentationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return status, apperr.Internal(err)
	}
	if presentationID != nil {
		status.PresentationExists = true
		var done int
		if err := s.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM presentation_exports
			WHERE presentation_id = $1 AND status = 'done'
		`, *presentationID).Scan(&done); err != nil {
			return status, apperr.Internal(err)
		}
		status.Exported = done > 0
	}

	switch {
	case ns.DeadlinePassed && !status.Exported:
		status.Warning = "Slide deadline has passed and slides are not exported yet"
	case ns.DeadlineHoursLeft < 24 && !status.Exported:
		status.Warning = "Slides not exported yet"
	case !status.PresentationExists:
		status.Warning = "No presentation built for this service yet"
	}
	return status, nil
}

// weekCounts counts records added in the last 7 days.
func (s *Service) weekCounts(ctx context.Context, out *Summary) error {
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM prayer_requests
		WHERE deleted_at IS NULL AND created_at > now() - interval '7 days'
	`).Scan(&out.PrayerThisWeek); err != nil {
		return apperr.Internal(err)
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM praise_reports
		WHERE deleted_at IS NULL AND created_at > now() - interval '7 days'
	`).Scan(&out.PraiseThisWeek); err != nil {
		return apperr.Internal(err)
	}
	return s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_summaries WHERE status = 'draft' AND deleted_at IS NULL
	`).Scan(&out.DraftSummaryCount)
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// Handler exposes the dashboard endpoint.
type Handler struct{ svc *Service }

// NewHandler builds the HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register mounts the dashboard route.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/dashboard/summary", auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor), h.summary)
	g.GET("/notifications", auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor, auth.RoleViewer), h.notifications)
	g.POST("/notifications/:id/read", auth.RequireRole(auth.RoleAdmin, auth.RoleIntercessor, auth.RoleViewer), h.markRead)
}

func (h *Handler) summary(c *gin.Context) {
	data, err := h.svc.Build(c.Request.Context(), c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, "Dashboard summary retrieved", data)
}

func (h *Handler) notifications(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT id, title, body, link, read_at, created_at
		FROM notifications WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 50
	`, c.GetString(middleware.CtxUserID))
	if err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var (
			id, title, body string
			link            *string
			readAt          *time.Time
			createdAt       time.Time
		)
		if err := rows.Scan(&id, &title, &body, &link, &readAt, &createdAt); err != nil {
			response.Fail(c, apperr.Internal(err))
			return
		}
		out = append(out, map[string]any{
			"id": id, "title": title, "body": body, "link": link,
			"read_at": readAt, "created_at": createdAt,
		})
	}
	response.OK(c, "Notifications retrieved", out)
}

func (h *Handler) markRead(c *gin.Context) {
	if _, err := h.svc.pool.Exec(c.Request.Context(), `
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL
	`, c.Param("id"), c.GetString(middleware.CtxUserID)); err != nil {
		response.Fail(c, apperr.Internal(err))
		return
	}
	response.OK(c, "Notification marked as read", gin.H{"read": true})
}
