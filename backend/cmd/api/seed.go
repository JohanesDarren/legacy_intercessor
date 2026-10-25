package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/legacy-sukawarna/legacy-intercessor/backend/internal/config"
)

// seedAccount is one development/demo account.
type seedAccount struct {
	Email    string
	FullName string
	Role     string
}

// seedUsers creates the initial accounts so the app is usable on first boot.
// In production this only creates the admin when no users exist yet; the demo
// intercessors require SEED_DEMO_USERS=true.
func seedUsers(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}

	accounts := []seedAccount{}

	if count == 0 {
		accounts = append(accounts, seedAccount{
			Email: "admin@legacy.sukawarna.id", FullName: "Lead Intercessor", Role: "admin",
		})
		log.Info("no users found, seeding the lead intercessor account")
	}

	if cfg.SeedDemoUsers {
		accounts = append(accounts,
			seedAccount{Email: "intercessor1@legacy.sukawarna.id", FullName: "Intercessor One", Role: "intercessor"},
			seedAccount{Email: "intercessor2@legacy.sukawarna.id", FullName: "Intercessor Two", Role: "intercessor"},
			seedAccount{Email: "intercessor3@legacy.sukawarna.id", FullName: "Intercessor Three", Role: "intercessor"},
			seedAccount{Email: "intercessor4@legacy.sukawarna.id", FullName: "Intercessor Four", Role: "intercessor"},
			seedAccount{Email: "media@legacy.sukawarna.id", FullName: "Media Volunteer", Role: "viewer"},
		)
	}

	if len(accounts) == 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.DefaultUserPassword), cfg.BcryptCost)
	if err != nil {
		return err
	}

	for _, a := range accounts {
		tag, err := pool.Exec(ctx, `
			INSERT INTO users (full_name, email, password_hash, role, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (email) DO NOTHING
		`, a.FullName, a.Email, string(hash), a.Role)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			log.Info("seeded account", slog.String("email", a.Email), slog.String("role", a.Role))
		}
	}
	return nil
}
