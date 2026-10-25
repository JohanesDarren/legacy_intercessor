// Package audit writes the accountability trail. Only field names are stored,
// never prayer content, names or follow-up notes (PRD 9.3).
package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is one audit record.
type Entry struct {
	ActorID  string
	Action   string
	Entity   string
	EntityID string
	Fields   []string
	IP       string
}

// Logger persists audit entries. Failures are logged but never block the
// business operation.
type Logger struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// New creates an audit logger.
func New(pool *pgxpool.Pool, log *slog.Logger) *Logger {
	return &Logger{pool: pool, log: log}
}

// Record writes an audit entry. `fields` must contain column names only.
func (l *Logger) Record(ctx context.Context, e Entry) {
	var meta []byte
	if len(e.Fields) > 0 {
		meta, _ = json.Marshal(map[string]any{"changed_fields": e.Fields})
	}

	var actor any
	if e.ActorID != "" {
		actor = e.ActorID
	}
	var entityID any
	if e.EntityID != "" {
		entityID = e.EntityID
	}
	var ip any
	if e.IP != "" {
		ip = e.IP
	}

	_, err := l.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, entity, entity_id, metadata, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, actor, e.Action, e.Entity, entityID, meta, ip)
	if err != nil {
		l.log.Warn("audit write failed", slog.String("action", e.Action), slog.Any("error", err))
	}
}
