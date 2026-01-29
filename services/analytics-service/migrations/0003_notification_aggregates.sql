ALTER TABLE notification_metrics
  ADD COLUMN IF NOT EXISTS business_id UUID;

CREATE INDEX IF NOT EXISTS notification_metrics_business_time_idx
  ON notification_metrics (business_id, sent_at);

CREATE TABLE IF NOT EXISTS daily_notification_metrics (
    business_id UUID NOT NULL,
    day DATE NOT NULL,
    channel VARCHAR(20) NOT NULL,
    sent_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, day, channel)
);
