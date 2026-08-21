package services

import (
	"context"
	"fmt"
	"hawking-bot/internal/repository"
	"strings"
)

type JadwalService struct {
	repo *repository.JadwalRepository
}

func NewJadwalService(repo *repository.JadwalRepository) *JadwalService {
	return &JadwalService{repo: repo}
}

func (s *JadwalService) GetFormattedJadwal(ctx context.Context) string {
	listJadwal, err := s.repo.GetAll(ctx)
	if err != nil {
		return err.Error()
	}

	if len(listJadwal) == 0 {
		return "📅 **Jadwal Kuliah:**\nBelum ada jadwal yang tersimpan."
	}

	var builder strings.Builder
	builder.WriteString("📅 **Jadwal Kuliah:**\n\n")
	currentDay := ""

	for _, j := range listJadwal {
		if currentDay != j.Hari {
			if currentDay != "" {
				builder.WriteString("\n")
			}
			builder.WriteString(fmt.Sprintf("**%s**\n", j.Hari))
			currentDay = j.Hari
		}

		waktuMulai := j.WaktuMulai.Format("15:04")
		waktuSelesai := j.WaktuSelesai.Format("15:04")
		builder.WriteString(fmt.Sprintf("• %s - %s | **%s** (%d SKS)\n", waktuMulai, waktuSelesai, j.MatKul, j.SKS))
		builder.WriteString(fmt.Sprintf("  📍 %s | 👨‍🏫 %s\n", j.Ruang, j.Dosen))
	}

	return builder.String()
}
