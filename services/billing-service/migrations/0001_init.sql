CREATE TABLE IF NOT EXISTS provider_events (
    id BIGSERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL DEFAULT 'local',
    provider_event_id TEXT NOT NULL,
    event_type VARCHAR(200) NOT NULL,
    payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_event_id)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    business_id UUID PRIMARY KEY,
    tier VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    provider VARCHAR(50) NOT NULL DEFAULT 'local',
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL DEFAULT uuid_generate_v4(),
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(200) NOT NULL,
    payload JSONB NOT NULL,
    traceparent TEXT,
    tracestate TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_unpublished
    ON outbox_events (published_at)
    WHERE published_at IS NULL;
