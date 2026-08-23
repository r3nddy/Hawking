package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"hawking-bot/internal/models"
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
	if err := r.db.QueryRowContext(ctx, query, jadwalID, reminderDate.Format("2006-01-02")).Scan(&count); err != nil {
		return false, fmt.Errorf("gagal mengecek status reminder: %w", err)
	}
	return count > 0, nil
}

func scanReminderConfig(scanner interface{ Scan(...any) error }) (models.ReminderConfig, error) {
	var config models.ReminderConfig
	var rawReminderTime any

	if err := scanner.Scan(
		&config.ID,
		&config.GuildID,
		&config.ChannelID,
		&rawReminderTime,
		&config.IsEnabled,
		&config.DaysBefore,
		&config.CreatedAt,
		&config.UpdatedAt,
	); err != nil {
		return config, err
	}

	reminderTime, err := parseReminderTime(rawReminderTime)
	if err != nil {
		return config, err
	}
	config.ReminderTime = reminderTime
	return config, nil
}

func parseReminderTime(value any) (time.Time, error) {
	var text string
	switch typed := value.(type) {
	case time.Time:
		return typed, nil
	case string:
		text = strings.TrimSpace(typed)
	case []byte:
		text = strings.TrimSpace(string(typed))
	default:
		return time.Time{}, fmt.Errorf("format reminder_time tidak didukung: %T", value)
	}

	for _, layout := range []string{"15:04:05.999999999", "15:04:05", "15:04"} {
		if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("format reminder_time tidak valid: %q", text)
}

// GetReminderConfig mengambil konfigurasi reminder untuk guild.
func (r *ReminderRepository) GetReminderConfig(ctx context.Context, guildID string) (*models.ReminderConfig, error) {
	query := `
		SELECT id, guild_id, channel_id, reminder_time, is_enabled, days_before, created_at, updated_at
		FROM reminder_config
		WHERE guild_id = $1
	`

	config, err := scanReminderConfig(r.db.QueryRowContext(ctx, query, guildID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil konfigurasi reminder: %w", err)
	}
	return &config, nil
}

// UpsertReminderConfig membuat atau memperbarui konfigurasi reminder.
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

	if err := r.db.QueryRowContext(ctx, query,
		config.GuildID,
		config.ChannelID,
		config.ReminderTime,
		config.IsEnabled,
		config.DaysBefore,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt); err != nil {
		return fmt.Errorf("gagal menyimpan konfigurasi reminder: %w", err)
	}
	return nil
}

// GetAllActiveConfigs mengambil semua konfigurasi reminder yang aktif.
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
		config, err := scanReminderConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca konfigurasi: %w", err)
		}
		configs = append(configs, config)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gagal membaca konfigurasi: %w", err)
	}
	return configs, nil
}

// GetReminderHistory mengambil riwayat reminder yang sudah dikirim.
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
		if err := rows.Scan(
			&reminder.ID,
			&reminder.JadwalID,
			&reminder.ReminderDate,
			&reminder.SentAt,
			&reminder.ChannelID,
			&reminder.MessageID,
			&reminder.Status,
		); err != nil {
			return nil, fmt.Errorf("gagal membaca data reminder: %w", err)
		}
		reminders = append(reminders, reminder)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gagal membaca data reminder: %w", err)
	}
	return reminders, nil
}
