package repository

import (
	"context"
	"database/sql"
	"fmt"
	"hawking-bot/internal/models"
	"time"
)

type ReminderRepository struct {
	db *sql.DB
}

func NewReminderRepository(db *sql.DB) *ReminderRepository {
	return &ReminderRepository{db: db}
}

// CreateReminder mencatat reminder yang sudah dikirim
func (r *ReminderRepository) CreateReminder(ctx context.Context, reminder *models.ClassReminder) error {
	query := `
		INSERT INTO class_reminders (jadwal_id, reminder_date, channel_id, message_id, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (jadwal_id, reminder_date) DO NOTHING
		RETURNING id, sent_at
	`

	err := r.db.QueryRowContext(ctx, query,
		reminder.JadwalID, reminder.ReminderDate, reminder.ChannelID, reminder.MessageID, reminder.Status,
	).Scan(&reminder.ID, &reminder.SentAt)

	if err == sql.ErrNoRows {
		// Reminder sudah ada (conflict), ini bukan error
		return nil
	}

	if err != nil {
		return fmt.Errorf("gagal mencatat reminder: %w", err)
	}

	return nil
}

// IsReminderSent mengecek apakah reminder untuk jadwal tertentu sudah dikirim
func (r *ReminderRepository) IsReminderSent(ctx context.Context, jadwalID int, reminderDate time.Time) (bool, error) {
	query := `
		SELECT COUNT(*) FROM class_reminders
		WHERE jadwal_id = $1 AND reminder_date = $2 AND status = 'sent'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, jadwalID, reminderDate.Format("2006-01-02")).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("gagal mengecek status reminder: %w", err)
	}

	return count > 0, nil
}

// GetReminderConfig mengambil konfigurasi reminder untuk guild
func (r *ReminderRepository) GetReminderConfig(ctx context.Context, guildID string) (*models.ReminderConfig, error) {
	query := `
		SELECT id, guild_id, channel_id, reminder_time, is_enabled, days_before, created_at, updated_at
		FROM reminder_config
		WHERE guild_id = $1
	`

	var config models.ReminderConfig
	err := r.db.QueryRowContext(ctx, query, guildID).Scan(
		&config.ID, &config.GuildID, &config.ChannelID, &config.ReminderTime,
		&config.IsEnabled, &config.DaysBefore, &config.CreatedAt, &config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Belum ada konfigurasi
	}

	if err != nil {
		return nil, fmt.Errorf("gagal mengambil konfigurasi reminder: %w", err)
	}

	return &config, nil
}

// UpsertReminderConfig membuat atau memperbarui konfigurasi reminder
func (r *ReminderRepository) UpsertReminderConfig(ctx context.Context, config *models.ReminderConfig) error {
	query := `
		INSERT INTO reminder_config (guild_id, channel_id, reminder_time, is_enabled, days_before)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guild_id) DO UPDATE SET
			channel_id = EXCLUDED.channel_id,
			reminder_time = EXCLUDED.reminder_time,
			is_enabled = EXCLUDED.is_enabled,
			days_before = EXCLUDED.days_before
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		config.GuildID, config.ChannelID, config.ReminderTime, config.IsEnabled, config.DaysBefore,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)

	if err != nil {
		return fmt.Errorf("gagal menyimpan konfigurasi reminder: %w", err)
	}

	return nil
}

// GetAllActiveConfigs mengambil semua konfigurasi reminder yang aktif
func (r *ReminderRepository) GetAllActiveConfigs(ctx context.Context) ([]models.ReminderConfig, error) {
	query := `
		SELECT id, guild_id, channel_id, reminder_time, is_enabled, days_before, created_at, updated_at
		FROM reminder_config
		WHERE is_enabled = true
		ORDER BY guild_id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil konfigurasi aktif: %w", err)
	}
	defer rows.Close()

	var configs []models.ReminderConfig
	for rows.Next() {
		var config models.ReminderConfig
		if err := rows.Scan(&config.ID, &config.GuildID, &config.ChannelID, &config.ReminderTime,
			&config.IsEnabled, &config.DaysBefore, &config.CreatedAt, &config.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gagal membaca konfigurasi: %w", err)
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// GetReminderHistory mengambil riwayat reminder yang sudah dikirim
func (r *ReminderRepository) GetReminderHistory(ctx context.Context, limit int) ([]models.ClassReminder, error) {
	query := `
		SELECT id, jadwal_id, reminder_date, sent_at, channel_id, message_id, status
		FROM class_reminders
		ORDER BY sent_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil riwayat reminder: %w", err)
	}
	defer rows.Close()

	var reminders []models.ClassReminder
	for rows.Next() {
		var reminder models.ClassReminder
		if err := rows.Scan(&reminder.ID, &reminder.JadwalID, &reminder.ReminderDate,
			&reminder.SentAt, &reminder.ChannelID, &reminder.MessageID, &reminder.Status); err != nil {
			return nil, fmt.Errorf("gagal membaca data reminder: %w", err)
		}
		reminders = append(reminders, reminder)
	}

	return reminders, rows.Err()
}
