-- Migration: Add display_name column to profile_schema and backfill with attribute_name
ALTER TABLE profile_schema ADD COLUMN IF NOT EXISTS display_name VARCHAR(255);
UPDATE profile_schema SET display_name = attribute_name WHERE display_name IS NULL;
