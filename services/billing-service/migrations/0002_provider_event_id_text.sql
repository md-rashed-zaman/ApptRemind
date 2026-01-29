ALTER TABLE provider_events
ALTER COLUMN provider_event_id TYPE TEXT
USING provider_event_id::text;

