CREATE TABLE IF NOT EXISTS business_profiles (
    business_id UUID PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS business_services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_id UUID NOT NULL,
    name TEXT NOT NULL,
    duration_minutes INT NOT NULL,
    price NUMERIC(12,2) NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_business_services_business
    ON business_services (business_id);

CREATE TABLE IF NOT EXISTS staff (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_id UUID NOT NULL,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_staff_business
    ON staff (business_id);

-- Weekday uses Go's time.Weekday numbering: Sunday=0 ... Saturday=6.
CREATE TABLE IF NOT EXISTS staff_working_hours (
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    weekday INT NOT NULL,
    is_working BOOLEAN NOT NULL DEFAULT true,
    start_minute INT NOT NULL DEFAULT 540, -- 09:00
    end_minute INT NOT NULL DEFAULT 1020,  -- 17:00
    PRIMARY KEY (staff_id, weekday)
);

