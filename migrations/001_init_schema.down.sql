-- Rollback: Drop all tables and functions created in 001_init_schema.up.sql

-- Drop triggers first
DROP TRIGGER IF EXISTS update_reminder_config_updated_at ON reminder_config;
DROP TRIGGER IF EXISTS update_jadwal_kelas_updated_at ON jadwal_kelas;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_reminders_status;
DROP INDEX IF EXISTS idx_reminders_date;
DROP INDEX IF EXISTS idx_jadwal_active;
DROP INDEX IF EXISTS idx_jadwal_hari;

-- Drop tables (in reverse order of dependencies)
DROP TABLE IF EXISTS class_reminders;
DROP TABLE IF EXISTS reminder_config;
DROP TABLE IF EXISTS jadwal_kelas;
DROP TABLE IF EXISTS authorized_users;
