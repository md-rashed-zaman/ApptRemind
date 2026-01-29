CREATE TABLE IF NOT EXISTS business_entitlements (
    business_id UUID PRIMARY KEY,
    tier VARCHAR(50) NOT NULL,
    max_monthly_appointments INT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

