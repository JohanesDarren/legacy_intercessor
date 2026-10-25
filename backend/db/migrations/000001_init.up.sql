-- ================================================================
-- Legacy Intercessor - initial schema
-- Matches PRD v1.0 Section 6.3
-- ================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ================================================================
-- TABLE: users
-- ================================================================
CREATE TABLE users (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name      VARCHAR(100) NOT NULL,
    email          VARCHAR(255) NOT NULL,
    password_hash  VARCHAR(255) NULL,
    role           VARCHAR(20)  NOT NULL DEFAULT 'intercessor'
                   CHECK (role IN ('admin', 'intercessor', 'viewer')),
    is_active      BOOLEAN      NOT NULL DEFAULT true,
    failed_logins  SMALLINT     NOT NULL DEFAULT 0,
    locked_until   TIMESTAMPTZ  NULL,
    last_login_at  TIMESTAMPTZ  NULL,
    privacy_ack_at TIMESTAMPTZ  NULL,
    deleted_at     TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by     UUID         NULL REFERENCES users(id),
    updated_by     UUID         NULL REFERENCES users(id),
    CONSTRAINT uq_users_email UNIQUE (email)
);

-- ================================================================
-- TABLE: user_invites
-- ================================================================
CREATE TABLE user_invites (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email        VARCHAR(255) NOT NULL,
    full_name    VARCHAR(100) NULL,
    role         VARCHAR(20)  NOT NULL CHECK (role IN ('admin', 'intercessor', 'viewer')),
    token_hash   VARCHAR(255) NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ  NOT NULL,
    accepted_at  TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by   UUID         NULL REFERENCES users(id)
);

-- ================================================================
-- TABLE: refresh_tokens
-- ================================================================
CREATE TABLE refresh_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ  NOT NULL,
    revoked_at  TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ================================================================
-- TABLE: categories
-- ================================================================
CREATE TABLE categories (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        VARCHAR(10) NOT NULL CHECK (kind IN ('prayer', 'praise')),
    name        VARCHAR(60) NOT NULL,
    sort_order  INT         NOT NULL DEFAULT 0,
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_categories_kind_name UNIQUE (kind, name)
);

-- ================================================================
-- TABLE: services
-- ================================================================
CREATE TABLE services (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    starts_at       TIMESTAMPTZ  NOT NULL,
    location        VARCHAR(150) NOT NULL DEFAULT 'GBI Aruna, Bandung',
    slide_deadline  TIMESTAMPTZ  NULL,
    notes           TEXT         NULL,
    deleted_at      TIMESTAMPTZ  NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by      UUID         NULL REFERENCES users(id),
    updated_by      UUID         NULL REFERENCES users(id),
    CONSTRAINT uq_services_starts_at UNIQUE (starts_at)
);

-- ================================================================
-- TABLE: prayer_requests
-- ================================================================
CREATE TABLE prayer_requests (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id        UUID         NULL REFERENCES services(id) ON DELETE RESTRICT,
    category_id       UUID         NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    requester_name    VARCHAR(100) NULL,
    requester_gender  VARCHAR(20)  NOT NULL DEFAULT 'unspecified'
                      CHECK (requester_gender IN ('male', 'female', 'unspecified')),
    title             VARCHAR(255) NOT NULL,
    content           TEXT         NOT NULL,
    is_urgent         BOOLEAN      NOT NULL DEFAULT false,
    status            VARCHAR(20)  NOT NULL DEFAULT 'open'
                      CHECK (status IN ('open', 'in_prayer', 'answered', 'closed')),
    display_consent   VARCHAR(20)  NOT NULL DEFAULT 'no'
                      CHECK (display_consent IN ('yes', 'anonymous', 'no')),
    received_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ  NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by        UUID         NULL REFERENCES users(id),
    updated_by        UUID         NULL REFERENCES users(id)
);

-- ================================================================
-- TABLE: follow_ups
-- ================================================================
CREATE TABLE follow_ups (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    prayer_request_id  UUID         NOT NULL REFERENCES prayer_requests(id) ON DELETE RESTRICT,
    contact_date       DATE         NOT NULL,
    channel            VARCHAR(20)  NOT NULL
                       CHECK (channel IN ('visit', 'phone', 'whatsapp', 'other')),
    update_type        VARCHAR(30)  NOT NULL
                       CHECK (update_type IN ('documentation', 'life_update', 'current_condition', 'other')),
    condition_status   VARCHAR(20)  NOT NULL
                       CHECK (condition_status IN ('improving', 'stable', 'worsening', 'answered', 'unreachable')),
    notes              TEXT         NOT NULL,
    next_action        VARCHAR(255) NULL,
    next_action_date   DATE         NULL,
    deleted_at         TIMESTAMPTZ  NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by         UUID         NULL REFERENCES users(id),
    updated_by         UUID         NULL REFERENCES users(id)
);

-- ================================================================
-- TABLE: praise_reports
-- ================================================================
CREATE TABLE praise_reports (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id          UUID         NULL REFERENCES services(id) ON DELETE RESTRICT,
    category_id         UUID         NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    source_follow_up_id UUID         NULL REFERENCES follow_ups(id) ON DELETE SET NULL,
    reporter_name       VARCHAR(100) NULL,
    reporter_gender     VARCHAR(20)  NOT NULL DEFAULT 'unspecified'
                        CHECK (reporter_gender IN ('male', 'female', 'unspecified')),
    title               VARCHAR(255) NOT NULL,
    content             TEXT         NOT NULL,
    is_featured         BOOLEAN      NOT NULL DEFAULT false,
    display_consent     VARCHAR(20)  NOT NULL DEFAULT 'no'
                        CHECK (display_consent IN ('yes', 'anonymous', 'no')),
    reported_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ  NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by          UUID         NULL REFERENCES users(id),
    updated_by          UUID         NULL REFERENCES users(id)
);

-- ================================================================
-- TABLE: ai_summaries
-- ================================================================
CREATE TABLE ai_summaries (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id      UUID         NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    category_id     UUID         NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    kind            VARCHAR(10)  NOT NULL CHECK (kind IN ('prayer', 'praise')),
    source_ids      UUID[]       NOT NULL,
    ai_text         TEXT         NOT NULL,
    final_text      TEXT         NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'approved', 'rejected')),
    model_name      VARCHAR(100) NOT NULL,
    prompt_version  VARCHAR(20)  NOT NULL,
    approved_by     UUID         NULL REFERENCES users(id),
    approved_at     TIMESTAMPTZ  NULL,
    deleted_at      TIMESTAMPTZ  NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by      UUID         NULL REFERENCES users(id),
    updated_by      UUID         NULL REFERENCES users(id)
);

-- ================================================================
-- TABLE: presentations
-- ================================================================
CREATE TABLE presentations (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id  UUID        NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    kind        VARCHAR(10) NOT NULL CHECK (kind IN ('prayer', 'praise')),
    version     INT         NOT NULL DEFAULT 1,
    status      VARCHAR(20) NOT NULL DEFAULT 'draft'
                CHECK (status IN ('draft', 'ready', 'exported')),
    slides      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID        NULL REFERENCES users(id),
    updated_by  UUID        NULL REFERENCES users(id),
    CONSTRAINT uq_presentation_version UNIQUE (service_id, kind, version)
);

-- ================================================================
-- TABLE: presentation_exports
-- ================================================================
CREATE TABLE presentation_exports (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    presentation_id  UUID        NOT NULL REFERENCES presentations(id) ON DELETE RESTRICT,
    format           VARCHAR(5)  NOT NULL CHECK (format IN ('png', 'pdf')),
    status           VARCHAR(20) NOT NULL DEFAULT 'queued'
                     CHECK (status IN ('queued', 'running', 'done', 'failed')),
    storage_key      TEXT        NULL,
    error_message    TEXT        NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       UUID        NULL REFERENCES users(id)
);

-- ================================================================
-- TABLE: booth_shifts / booth_assignments / swap_requests
-- ================================================================
CREATE TABLE booth_shifts (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id   UUID         NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    label        VARCHAR(50)  NOT NULL,
    starts_at    TIMESTAMPTZ  NOT NULL,
    ends_at      TIMESTAMPTZ  NOT NULL,
    capacity     SMALLINT     NOT NULL DEFAULT 2 CHECK (capacity BETWEEN 1 AND 4),
    is_published BOOLEAN      NOT NULL DEFAULT false,
    deleted_at   TIMESTAMPTZ  NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by   UUID         NULL REFERENCES users(id),
    updated_by   UUID         NULL REFERENCES users(id),
    CONSTRAINT chk_booth_shift_time CHECK (ends_at > starts_at)
);

CREATE TABLE booth_assignments (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    shift_id           UUID        NOT NULL REFERENCES booth_shifts(id) ON DELETE RESTRICT,
    user_id            UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status             VARCHAR(20) NOT NULL DEFAULT 'assigned'
                       CHECK (status IN ('assigned', 'swap_requested', 'cancelled')),
    swap_with_user_id  UUID        NULL REFERENCES users(id),
    deleted_at         TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by         UUID        NULL REFERENCES users(id),
    updated_by         UUID        NULL REFERENCES users(id)
);

CREATE UNIQUE INDEX uq_booth_assignment_active
    ON booth_assignments(shift_id, user_id) WHERE deleted_at IS NULL;

CREATE TABLE swap_requests (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id     UUID        NOT NULL REFERENCES booth_assignments(id) ON DELETE RESTRICT,
    requested_by      UUID        NOT NULL REFERENCES users(id),
    target_user_id    UUID        NOT NULL REFERENCES users(id),
    status            VARCHAR(20) NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'approved', 'declined', 'cancelled')),
    decided_by        UUID        NULL REFERENCES users(id),
    decided_at        TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ================================================================
-- TABLE: notifications (in-app, for swap decisions and roster news)
-- ================================================================
CREATE TABLE notifications (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       VARCHAR(150) NOT NULL,
    body        VARCHAR(500) NOT NULL,
    link        VARCHAR(200) NULL,
    read_at     TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ================================================================
-- TABLE: audit_logs
-- ================================================================
CREATE TABLE audit_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID        NULL REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(50) NOT NULL,
    entity      VARCHAR(50) NOT NULL,
    entity_id   UUID        NULL,
    metadata    JSONB       NULL,
    ip_address  INET        NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ================================================================
-- INDEXES
-- ================================================================
CREATE INDEX idx_prayer_requests_service  ON prayer_requests(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_prayer_requests_category ON prayer_requests(category_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_prayer_requests_urgent   ON prayer_requests(is_urgent, received_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_praise_reports_service   ON praise_reports(service_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_follow_ups_request       ON follow_ups(prayer_request_id, contact_date DESC);
CREATE INDEX idx_booth_assignments_user   ON booth_assignments(user_id);
CREATE INDEX idx_audit_logs_entity        ON audit_logs(entity, entity_id);
CREATE INDEX idx_ai_summaries_service     ON ai_summaries(service_id, kind) WHERE deleted_at IS NULL;
CREATE INDEX idx_refresh_tokens_user      ON refresh_tokens(user_id);
CREATE INDEX idx_notifications_user       ON notifications(user_id, created_at DESC);
