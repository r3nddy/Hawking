-- Migration: Initial schema for jadwal kelas and reminder system
-- Created: 2026-08-20

-- Table: authorized_users (sudah ada, ini untuk referensi)
CREATE TABLE IF NOT EXISTS authorized_users (
    discord_id VARCHAR(255) PRIMARY KEY,
    granted_by VARCHAR(255) NOT NULL,
    granted_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Table: jadwal_kelas
-- Menyimpan jadwal kuliah dengan informasi lengkap
CREATE TABLE IF NOT EXISTS jadwal_kelas (
    id SERIAL PRIMARY KEY,
    hari VARCHAR(20) NOT NULL CHECK (hari IN ('Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu', 'Minggu')),
    waktu_mulai TIME NOT NULL,
    waktu_selesai TIME NOT NULL,
    ruang VARCHAR(100) NOT NULL,
    matkul VARCHAR(255) NOT NULL,
    dosen VARCHAR(255) NOT NULL,
    semester VARCHAR(50) NOT NULL,
    sks INTEGER NOT NULL CHECK (sks > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT check_waktu CHECK (waktu_selesai > waktu_mulai)
);

-- Table: class_reminders
-- Menyimpan log reminder yang sudah dikirim untuk mencegah duplikasi
CREATE TABLE IF NOT EXISTS class_reminders (
    id SERIAL PRIMARY KEY,
    jadwal_id INTEGER NOT NULL REFERENCES jadwal_kelas(id) ON DELETE CASCADE,
    reminder_date DATE NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT NOW(),
    channel_id VARCHAR(255) NOT NULL,
    message_id VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'sent' CHECK (status IN ('sent', 'failed', 'scheduled')),
    UNIQUE (jadwal_id, reminder_date)
);

-- Table: reminder_config
-- Konfigurasi untuk reminder (channel Discord, waktu kirim, dll)
CREATE TABLE IF NOT EXISTS reminder_config (
    id SERIAL PRIMARY KEY,
    guild_id VARCHAR(255) NOT NULL,
    channel_id VARCHAR(255) NOT NULL,
    reminder_time TIME NOT NULL DEFAULT '20:00:00', -- Jam 8 malam
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    days_before INTEGER NOT NULL DEFAULT 1 CHECK (days_before >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (guild_id)
);

-- Indexes untuk performa query
CREATE INDEX IF NOT EXISTS idx_jadwal_hari ON jadwal_kelas(hari);
CREATE INDEX IF NOT EXISTS idx_jadwal_active ON jadwal_kelas(is_active);
CREATE INDEX IF NOT EXISTS idx_reminders_date ON class_reminders(reminder_date);
CREATE INDEX IF NOT EXISTS idx_reminders_status ON class_reminders(status);

-- Trigger untuk auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_jadwal_kelas_updated_at BEFORE UPDATE ON jadwal_kelas
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reminder_config_updated_at BEFORE UPDATE ON reminder_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

