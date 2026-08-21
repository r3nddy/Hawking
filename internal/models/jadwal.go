package models

import "time"

type Jadwal struct {
	ID           int       `json:"id"`
	Hari         string    `json:"hari"`
	WaktuMulai   time.Time `json:"waktu_mulai"`
	WaktuSelesai time.Time `json:"waktu_selesai"`
	Ruang        string    `json:"ruang"`
	MatKul       string    `json:"matkul"`
	Dosen        string    `json:"dosen"`
	Semester     string    `json:"semester"`
	SKS          int       `json:"sks"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ClassReminder struct {
	ID           int       `json:"id"`
	JadwalID     int       `json:"jadwal_id"`
	ReminderDate time.Time `json:"reminder_date"`
	SentAt       time.Time `json:"sent_at"`
	ChannelID    string    `json:"channel_id"`
	MessageID    *string   `json:"message_id,omitempty"`
	Status       string    `json:"status"` // sent, failed, scheduled
}

type ReminderConfig struct {
	ID           int       `json:"id"`
	GuildID      string    `json:"guild_id"`
	ChannelID    string    `json:"channel_id"`
	ReminderTime time.Time `json:"reminder_time"`
	IsEnabled    bool      `json:"is_enabled"`
	DaysBefore   int       `json:"days_before"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
