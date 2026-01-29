ALTER TABLE appointments
    ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancellation_reason TEXT;

CREATE TABLE IF NOT EXISTS booking_idempotency_keys (
    business_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL,
    appointment_id UUID REFERENCES appointments(id),
    status_code INT,
    response_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_booking_idem_created_at
    ON booking_idempotency_keys (created_at);
