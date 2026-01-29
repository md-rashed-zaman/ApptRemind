-- Prevent overlapping time-off ranges per staff member.
-- Uses an exclusion constraint over tstzrange(start_time, end_time).
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE staff_time_off
  ADD CONSTRAINT staff_time_off_no_overlap
  EXCLUDE USING gist (
    staff_id WITH =,
    tstzrange(start_time, end_time, '[)') WITH &&
  );

