ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS provider VARCHAR(50) NOT NULL DEFAULT 'local',
ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT,
ADD COLUMN IF NOT EXISTS stripe_subscription_id TEXT;

CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_subscription_id
    ON subscriptions (stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;

