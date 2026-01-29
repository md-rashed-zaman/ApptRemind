CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(200) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    actor_id TEXT,
    business_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_business_id
    ON audit_events (business_id);

CREATE INDEX IF NOT EXISTS idx_audit_events_event_type
    ON audit_events (event_type);

CREATE INDEX IF NOT EXISTS idx_audit_events_created_at
    ON audit_events (created_at DESC);
