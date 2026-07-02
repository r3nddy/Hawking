package repository

import (
	"database/sql"
	"fmt"
	"hawking-bot/internal/models"
)

type JadwalRepository struct {
	db *sql.DB
}

func NewJadwalRepository(db *sql.DB) *JadwalRepository {
	return &JadwalRepository{db: db}
}

func (r *JadwalRepository) GetAll() ([]models.Jadwal, error) {
	rows, err := r.db.Query("SELECT hari, waktu, ruang, matkul, dosen, semester, sks FROM jadwal ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil jadwal dari database: %w", err)
	}
	defer rows.Close()

	var listJadwal []models.Jadwal
	for rows.Next() {
		var j models.Jadwal
		if err := rows.Scan(&j.Hari, &j.Waktu, &j.Ruang, &j.MatKul, &j.Dosen, &j.Semester, &j.SKS); err != nil {
			return nil, fmt.Errorf("gagal membaca data jadwal: %w", err)
		}
		listJadwal = append(listJadwal, j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error setelah membaca jadwal: %w", err)
	}
	return listJadwal, nil
}
