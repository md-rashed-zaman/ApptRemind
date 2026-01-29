CREATE TABLE IF NOT EXISTS inbox_events (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(200) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id)
);

CREATE TABLE IF NOT EXISTS notification_metrics (
    id BIGSERIAL PRIMARY KEY,
    appointment_id UUID NOT NULL,
    channel VARCHAR(20) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'sent',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scheduler_dlq_events (
    id BIGSERIAL PRIMARY KEY,
    appointment_id UUID NOT NULL,
    business_id UUID NOT NULL,
    channel VARCHAR(20) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    remind_at TIMESTAMPTZ NOT NULL,
    error_reason TEXT NOT NULL,
    failed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS security_audit_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    actor_id UUID,
    metadata JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
