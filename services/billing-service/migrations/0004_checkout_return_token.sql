ALTER TABLE checkout_sessions
ADD COLUMN IF NOT EXISTS return_token TEXT,
ADD COLUMN IF NOT EXISTS return_seen_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_checkout_sessions_return_token
    ON checkout_sessions (stripe_session_id, return_token);

