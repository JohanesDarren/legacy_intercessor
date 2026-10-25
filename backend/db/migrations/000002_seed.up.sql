-- ================================================================
-- Seed: categories, the four intercessors, and the next services
-- Idempotent: safe to run more than once.
-- ================================================================

INSERT INTO categories (kind, name, sort_order) VALUES
    ('prayer', 'Health', 1),
    ('prayer', 'Family', 2),
    ('prayer', 'Finance', 3),
    ('prayer', 'Work', 4),
    ('prayer', 'Spiritual', 5),
    ('prayer', 'Relationship', 6),
    ('prayer', 'Other', 7),
    ('praise', 'Healing', 1),
    ('praise', 'Provision', 2),
    ('praise', 'Breakthrough', 3),
    ('praise', 'Salvation', 4),
    ('praise', 'Family', 5),
    ('praise', 'Other', 6)
ON CONFLICT (kind, name) DO NOTHING;

-- ----------------------------------------------------------------
-- Users are seeded by the API on first boot (see internal/users/seed.go)
-- so that passwords are hashed with bcrypt cost 12 from the environment.
-- Default accounts (development only, see .env.example):
--   admin@legacy.sukawarna.id      / LegacyIntercessor#2026
--   intercessor1..4@legacy...      / LegacyIntercessor#2026
-- ----------------------------------------------------------------

-- Next four Saturday 17:00 WIB services, only when none exist yet.
INSERT INTO services (starts_at, location, slide_deadline, notes)
SELECT d::timestamptz, 'GBI Aruna, Bandung', (d::date + time '12:00')::timestamptz, 'Saturday 17:00 service'
FROM (
    SELECT (date_trunc('week', now() AT TIME ZONE 'Asia/Jakarta') + (5 - 1) * interval '1 day' + interval '17 hours')
             AT TIME ZONE 'Asia/Jakarta' AS d
) base
WHERE NOT EXISTS (SELECT 1 FROM services)
ON CONFLICT (starts_at) DO NOTHING;

INSERT INTO services (starts_at, location, slide_deadline, notes)
SELECT s.starts_at + interval '7 days',
       s.location,
       s.slide_deadline + interval '7 days',
       s.notes
FROM services s
WHERE s.starts_at < now() + interval '21 days'
  AND NOT EXISTS (
      SELECT 1 FROM services x WHERE x.starts_at = s.starts_at + interval '7 days'
  );
