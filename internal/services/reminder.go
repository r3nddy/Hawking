package services

import (
	"context"
	"fmt"
	"hawking-bot/internal/models"
	"hawking-bot/internal/repository"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type ReminderService struct {
	jadwalRepo   *repository.JadwalRepository
	reminderRepo *repository.ReminderRepository
	session      *discordgo.Session
}

func NewReminderService(
	jadwalRepo *repository.JadwalRepository,
	reminderRepo *repository.ReminderRepository,
	session *discordgo.Session,
) *ReminderService {
	return &ReminderService{
		jadwalRepo:   jadwalRepo,
		reminderRepo: reminderRepo,
		session:      session,
	}
}

// CheckAndSendReminders memeriksa jadwal besok dan mengirim reminder jika belum dikirim
func (s *ReminderService) CheckAndSendReminders(ctx context.Context) error {
	// Ambil semua konfigurasi aktif
	configs, err := s.reminderRepo.GetAllActiveConfigs(ctx)
	if err != nil {
		return fmt.Errorf("gagal mengambil konfigurasi: %w", err)
	}

	if len(configs) == 0 {
		return nil // Tidak ada konfigurasi aktif
	}

	// Ambil jadwal untuk besok
	schedules, err := s.jadwalRepo.GetScheduleForTomorrow(ctx)
	if err != nil {
		return fmt.Errorf("gagal mengambil jadwal besok: %w", err)
	}

	if len(schedules) == 0 {
		return nil // Tidak ada jadwal besok
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	reminderDate := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.Local)

	// Kirim reminder untuk setiap konfigurasi aktif
	for _, config := range configs {
		if err := s.sendReminderForConfig(ctx, config, schedules, reminderDate); err != nil {
			// Log error tapi lanjutkan untuk config lain
			fmt.Printf("Error sending reminder for guild %s: %v\n", config.GuildID, err)
		}
	}

	return nil
}

// sendReminderForConfig mengirim reminder untuk satu guild/channel
func (s *ReminderService) sendReminderForConfig(
	ctx context.Context,
	config models.ReminderConfig,
	schedules []models.Jadwal,
	reminderDate time.Time,
) error {
	// Cek apakah sudah ada reminder yang dikirim untuk jadwal ini
	alreadySent := make(map[int]bool)
	for _, schedule := range schedules {
		sent, err := s.reminderRepo.IsReminderSent(ctx, schedule.ID, reminderDate)
		if err != nil {
			return fmt.Errorf("gagal cek status reminder: %w", err)
		}
		alreadySent[schedule.ID] = sent
	}

	// Filter jadwal yang belum dikirim remindernya
	var unsent []models.Jadwal
	for _, schedule := range schedules {
		if !alreadySent[schedule.ID] {
			unsent = append(unsent, schedule)
		}
	}

	if len(unsent) == 0 {
		return nil // Semua reminder sudah dikirim
	}

	// Format pesan reminder
	message := s.formatReminderMessage(unsent, reminderDate)

	// Kirim pesan ke Discord
	sentMessage, err := s.session.ChannelMessageSendComplex(config.ChannelID, &discordgo.MessageSend{
		Content: message,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeEveryone},
		},
	})

	if err != nil {
		// Catat sebagai failed
		for _, schedule := range unsent {
			reminder := &models.ClassReminder{
				JadwalID:     schedule.ID,
				ReminderDate: reminderDate,
				ChannelID:    config.ChannelID,
				Status:       "failed",
			}
			_ = s.reminderRepo.CreateReminder(ctx, reminder)
		}
		return fmt.Errorf("gagal mengirim pesan ke Discord: %w", err)
	}

	// Catat reminder yang berhasil dikirim
	for _, schedule := range unsent {
		reminder := &models.ClassReminder{
			JadwalID:     schedule.ID,
			ReminderDate: reminderDate,
			ChannelID:    config.ChannelID,
			MessageID:    &sentMessage.ID,
			Status:       "sent",
		}
		if err := s.reminderRepo.CreateReminder(ctx, reminder); err != nil {
			fmt.Printf("Warning: gagal mencatat reminder untuk jadwal %d: %v\n", schedule.ID, err)
		}
	}

	return nil
}

// formatReminderMessage membuat pesan reminder yang menarik
func (s *ReminderService) formatReminderMessage(schedules []models.Jadwal, reminderDate time.Time) string {
	var builder strings.Builder

	// Header dengan emoji dan mention everyone
	builder.WriteString("@everyone\n\n")
	builder.WriteString("🔔 **REMINDER JADWAL KULIAH BESOK** 🔔\n\n")

	// Format tanggal Indonesia
	days := map[time.Weekday]string{
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
		time.Saturday:  "Sabtu",
		time.Sunday:    "Minggu",
	}

	months := map[time.Month]string{
		time.January:   "Januari",
		time.February:  "Februari",
		time.March:     "Maret",
		time.April:     "April",
		time.May:       "Mei",
		time.June:      "Juni",
		time.July:      "Juli",
		time.August:    "Agustus",
		time.September: "September",
		time.October:   "Oktober",
		time.November:  "November",
		time.December:  "Desember",
	}

	hari := days[reminderDate.Weekday()]
	builder.WriteString(fmt.Sprintf("📅 **%s, %d %s %d**\n\n",
		hari,
		reminderDate.Day(),
		months[reminderDate.Month()],
		reminderDate.Year(),
	))

	// Daftar jadwal
	builder.WriteString("**Mata Kuliah:**\n")
	for i, schedule := range schedules {
		waktuMulai := schedule.WaktuMulai.Format("15:04")
		waktuSelesai := schedule.WaktuSelesai.Format("15:04")

		builder.WriteString(fmt.Sprintf("\n**%d. %s** (%d SKS)\n", i+1, schedule.MatKul, schedule.SKS))
		builder.WriteString(fmt.Sprintf("   ⏰ %s - %s\n", waktuMulai, waktuSelesai))
		builder.WriteString(fmt.Sprintf("   📍 %s\n", schedule.Ruang))
		builder.WriteString(fmt.Sprintf("   👨‍🏫 %s\n", schedule.Dosen))
	}

	// Footer motivasi
	builder.WriteString("\n💡 **Tips:** Siapkan bahan kuliah dari malam ini ya!\n")
	builder.WriteString("📚 Jangan lupa cek tugas dan materi yang perlu dibawa.\n\n")
	builder.WriteString("_Semangat kuliahnya! 🎓_")

	return builder.String()
}

// SetupReminderConfig mengatur konfigurasi reminder untuk guild
func (s *ReminderService) SetupReminderConfig(ctx context.Context, config *models.ReminderConfig) error {
	return s.reminderRepo.UpsertReminderConfig(ctx, config)
}

// GetReminderConfig mengambil konfigurasi reminder untuk guild
func (s *ReminderService) GetReminderConfig(ctx context.Context, guildID string) (*models.ReminderConfig, error) {
	return s.reminderRepo.GetReminderConfig(ctx, guildID)
}

// ToggleReminder mengaktifkan/menonaktifkan reminder
func (s *ReminderService) ToggleReminder(ctx context.Context, guildID string, enabled bool) error {
	config, err := s.reminderRepo.GetReminderConfig(ctx, guildID)
	if err != nil {
		return err
	}

	if config == nil {
		return fmt.Errorf("konfigurasi reminder belum dibuat untuk guild ini")
	}

	config.IsEnabled = enabled
	return s.reminderRepo.UpsertReminderConfig(ctx, config)
}
