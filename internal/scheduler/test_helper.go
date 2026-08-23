package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"hawking-bot/internal/models"
	"hawking-bot/internal/services"
)

// TestScheduler adalah utility untuk testing scheduler tanpa menunggu cron
type TestScheduler struct {
	reminderSvc *services.ReminderService
}

// NewTestScheduler membuat instance test scheduler
func NewTestScheduler(reminderSvc *services.ReminderService) *TestScheduler {
	return &TestScheduler{
		reminderSvc: reminderSvc,
	}
}

// SimulateReminderCheck mensimulasikan pengecekan dan pengiriman reminder
func (t *TestScheduler) SimulateReminderCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("=== SIMULASI REMINDER CHECK ===")
	log.Println("Waktu sekarang:", time.Now().Format("2006-01-02 15:04:05"))

	tomorrow := time.Now().Add(24 * time.Hour)
	log.Printf("Mencari jadwal untuk: %s (%s)\n",
		tomorrow.Format("2006-01-02"),
		getDayInIndonesian(tomorrow.Weekday()),
	)

	result, err := t.reminderSvc.CheckAndSendReminders(ctx)
	if err != nil {
		return fmt.Errorf("error saat simulasi: %w", err)
	}
	log.Printf("Hasil: %d config aktif, %d jadwal, %d pesan terkirim, %d dilewati",
		result.ActiveConfigs, result.SchedulesFound, result.MessagesSent, result.Skipped)

	log.Println("=== SIMULASI SELESAI ===")
	return nil
}

func (t *TestScheduler) TestFormatMessage(schedules []models.Jadwal) string {
	if len(schedules) == 0 {
		return "Tidak ada jadwal untuk diformat"
	}

	tomorrow := time.Now().Add(24 * time.Hour)
	reminderDate := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.Local)

	message := formatTestMessage(schedules, reminderDate)
	return message
}

func formatTestMessage(schedules []models.Jadwal, reminderDate time.Time) string {
	var builder strings.Builder

	builder.WriteString("@everyone\n\n")
	builder.WriteString("🔔 **REMINDER JADWAL KULIAH BESOK** 🔔\n\n")

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
	fmt.Fprintf(&builder, "📅 **%s, %d %s %d**\n\n",
		hari,
		reminderDate.Day(),
		months[reminderDate.Month()],
		reminderDate.Year(),
	)

	builder.WriteString("**Mata Kuliah:**\n")
	for i, schedule := range schedules {
		waktuMulai := schedule.WaktuMulai.Format("15:04")
		waktuSelesai := schedule.WaktuSelesai.Format("15:04")

		fmt.Fprintf(&builder, "\n**%d. %s** (%d SKS)\n", i+1, schedule.MatKul, schedule.SKS)
		fmt.Fprintf(&builder, "   ⏰ %s - %s\n", waktuMulai, waktuSelesai)
		fmt.Fprintf(&builder, "   📍 %s\n", schedule.Ruang)
		fmt.Fprintf(&builder, "   👨‍🏫 %s\n", schedule.Dosen)
	}

	builder.WriteString("\n💡 **Tips:** Siapkan bahan kuliah dari malam ini ya!\n")
	builder.WriteString("📚 Jangan lupa cek tugas dan materi yang perlu dibawa.\n\n")
	builder.WriteString("_Semangat kuliahnya! 🎓_")

	return builder.String()
}

func getDayInIndonesian(day time.Weekday) string {
	days := map[time.Weekday]string{
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
		time.Saturday:  "Sabtu",
		time.Sunday:    "Minggu",
	}
	return days[day]
}
