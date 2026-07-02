package services

import (
	"fmt"
	"hawking-bot/internal/repository"
)

type JadwalService struct {
	repo *repository.JadwalRepository
}

func NewJadwalService(repo *repository.JadwalRepository) *JadwalService {
	return &JadwalService{repo: repo}
}

func (s *JadwalService) GetFormattedJadwal() string {
	listJadwal, err := s.repo.GetAll()
	if err != nil {
		return err.Error()
	}

	if len(listJadwal) == 0 {
		return "📅 **Jadwal Kuliah:**\nBelum ada jadwal yang tersimpan."
	}

	response := "📅 **Jadwal Kuliah:**\n"
	for _, j := range listJadwal {
		response += fmt.Sprintf("- %s: %s (%s)\n", j.Hari, j.MatKul, j.Waktu)
	}
	return response
}
