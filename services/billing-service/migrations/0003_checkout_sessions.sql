CREATE TABLE IF NOT EXISTS checkout_sessions (
    stripe_session_id TEXT PRIMARY KEY,
    business_id UUID NOT NULL,
    tier VARCHAR(50) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'created', -- created | completed | expired | canceled
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT,
    url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_checkout_sessions_business
    ON checkout_sessions (business_id, created_at DESC);

