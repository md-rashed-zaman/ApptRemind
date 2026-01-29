ALTER TABLE business_profiles
ADD COLUMN IF NOT EXISTS reminder_offsets_minutes INT[] NOT NULL DEFAULT '{1440,60}';

