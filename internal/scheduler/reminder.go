package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"hawking-bot/internal/services"

	"github.com/robfig/cron/v3"
)

type ReminderScheduler struct {
	cron            *cron.Cron
	reminderSvc     *services.ReminderService
	isRunning       bool
	defaultSchedule string
}

func NewReminderScheduler(reminderSvc *services.ReminderService) *ReminderScheduler {
	return &ReminderScheduler{
		cron:            cron.New(),
		reminderSvc:     reminderSvc,
		defaultSchedule: "0 20 * * *", // 8 PM setiap hari (jam 20:00)
	}
}

// Start menjalankan scheduler
func (s *ReminderScheduler) Start() error {
	if s.isRunning {
		return fmt.Errorf("scheduler sudah berjalan")
	}

	_, err := s.cron.AddFunc(s.defaultSchedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Println("[Scheduler] Memeriksa jadwal untuk reminder...")
		result, err := s.reminderSvc.CheckAndSendReminders(ctx)
		if err != nil {
			log.Printf("[Scheduler] Error mengirim reminder: %v\n", err)
		}
		if result != nil {
			log.Printf("[Scheduler] Check selesai: %d config aktif, %d jadwal, %d pesan terkirim, %d dilewati\n",
				result.ActiveConfigs, result.SchedulesFound, result.MessagesSent, result.Skipped)
		}
	})
	if err != nil {
		return fmt.Errorf("gagal menambahkan job ke scheduler: %w", err)
	}

	s.cron.Start()
	s.isRunning = true
	log.Printf("[Scheduler] Reminder scheduler started dengan schedule: %s\n", s.defaultSchedule)
	return nil
}

// Stop menghentikan scheduler
func (s *ReminderScheduler) Stop() {
	if !s.isRunning {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.isRunning = false
	log.Println("[Scheduler] Reminder scheduler stopped")
}

func (s *ReminderScheduler) SetSchedule(schedule string) error {
	if s.isRunning {
		return fmt.Errorf("hentikan scheduler terlebih dahulu sebelum mengubah schedule")
	}

	s.defaultSchedule = schedule
	return nil
}

func (s *ReminderScheduler) RunNow(ctx context.Context) error {
	log.Println("[Scheduler] Menjalankan check reminder manual...")
	result, err := s.reminderSvc.CheckAndSendReminders(ctx)
	if err != nil {
		return fmt.Errorf("gagal menjalankan reminder: %w", err)
	}
	log.Printf("[Scheduler] Check selesai: %d config aktif, %d jadwal, %d pesan terkirim, %d dilewati",
		result.ActiveConfigs, result.SchedulesFound, result.MessagesSent, result.Skipped)
	return nil
}

// IsRunning mengembalikan status scheduler
func (s *ReminderScheduler) IsRunning() bool {
	return s.isRunning
}

// GetNextRun mengembalikan waktu eksekusi berikutnya
func (s *ReminderScheduler) GetNextRun() time.Time {
	entries := s.cron.Entries()
	if len(entries) > 0 {
		return entries[0].Next
	}
	return time.Time{}
}
