CREATE TABLE IF NOT EXISTS booking_events (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL,
    event_type VARCHAR(200) NOT NULL,
    business_id UUID NOT NULL,
    appointment_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS booking_events_business_time_idx
    ON booking_events (business_id, occurred_at);

CREATE TABLE IF NOT EXISTS daily_appointment_metrics (
    business_id UUID NOT NULL,
    day DATE NOT NULL,
    booked_count INT NOT NULL DEFAULT 0,
    canceled_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, day)
);
