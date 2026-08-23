package services

import (
	"context"
	"errors"
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

type ReminderCheckResult struct {
	ActiveConfigs  int
	SchedulesFound int
	MessagesSent   int
	Skipped        int
	Failures       []string
}

// CheckAndSendReminders memeriksa jadwal sesuai konfigurasi dan mengirim reminder.
func (s *ReminderService) CheckAndSendReminders(ctx context.Context) (*ReminderCheckResult, error) {
	configs, err := s.reminderRepo.GetAllActiveConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil konfigurasi: %w", err)
	}

	result := &ReminderCheckResult{ActiveConfigs: len(configs)}
	for _, config := range configs {
		reminderDate := time.Now().AddDate(0, 0, config.DaysBefore)
		reminderDate = time.Date(reminderDate.Year(), reminderDate.Month(), reminderDate.Day(), 0, 0, 0, 0, time.Local)

		schedules, err := s.jadwalRepo.GetScheduleForDate(ctx, reminderDate)
		if err != nil {
			return nil, fmt.Errorf("gagal mengambil jadwal untuk guild %s: %w", config.GuildID, err)
		}
		result.SchedulesFound += len(schedules)
		if len(schedules) == 0 {
			continue
		}

		sent, skipped, err := s.sendReminderForConfig(ctx, config, schedules, reminderDate)
		if err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("guild %s: %v", config.GuildID, err))
			continue
		}
		if sent {
			result.MessagesSent++
		}
		if skipped {
			result.Skipped++
		}
	}

	if len(result.Failures) > 0 {
		return result, errors.New(strings.Join(result.Failures, "; "))
	}
	return result, nil
}

// sendReminderForConfig mengirim reminder untuk satu guild/channel.
func (s *ReminderService) sendReminderForConfig(
	ctx context.Context,
	config models.ReminderConfig,
	schedules []models.Jadwal,
	reminderDate time.Time,
) (sent bool, skipped bool, err error) {
	alreadySent := make(map[int]bool)
	for _, schedule := range schedules {
		isSent, checkErr := s.reminderRepo.IsReminderSent(ctx, schedule.ID, reminderDate)
		if checkErr != nil {
			return false, false, fmt.Errorf("gagal cek status reminder: %w", checkErr)
		}
		alreadySent[schedule.ID] = isSent
	}

	var unsent []models.Jadwal
	for _, schedule := range schedules {
		if !alreadySent[schedule.ID] {
			unsent = append(unsent, schedule)
		}
	}
	if len(unsent) == 0 {
		return false, true, nil
	}

	message := s.formatReminderMessage(unsent, reminderDate)
	sentMessage, err := s.session.ChannelMessageSendComplex(config.ChannelID, &discordgo.MessageSend{
		Content: message,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeEveryone},
		},
	})
	if err != nil {
		for _, schedule := range unsent {
			reminder := &models.ClassReminder{
				JadwalID: schedule.ID, ReminderDate: reminderDate,
				ChannelID: config.ChannelID, Status: "failed",
			}
			_ = s.reminderRepo.CreateReminder(ctx, reminder)
		}
		return false, false, fmt.Errorf("gagal mengirim pesan ke Discord: %w", err)
	}

	for _, schedule := range unsent {
		reminder := &models.ClassReminder{
			JadwalID: schedule.ID, ReminderDate: reminderDate,
			ChannelID: config.ChannelID, MessageID: &sentMessage.ID, Status: "sent",
		}
		if err := s.reminderRepo.CreateReminder(ctx, reminder); err != nil {
			return true, false, fmt.Errorf("pesan terkirim tetapi gagal mencatat reminder untuk jadwal %d: %w", schedule.ID, err)
		}
	}
	return true, false, nil
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
