package repository

import (
	"context"
	"database/sql"
	"fmt"
	"hawking-bot/internal/models"
	"time"
)

type JadwalRepository struct {
	db *sql.DB
}

func NewJadwalRepository(db *sql.DB) *JadwalRepository {
	return &JadwalRepository{db: db}
}

// GetAll mengambil semua jadwal aktif
func (r *JadwalRepository) GetAll(ctx context.Context) ([]models.Jadwal, error) {
	query := `
		SELECT id, hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks, is_active, created_at, updated_at
		FROM jadwal_kelas
		WHERE is_active = true
		ORDER BY
			CASE hari
				WHEN 'Senin' THEN 1
				WHEN 'Selasa' THEN 2
				WHEN 'Rabu' THEN 3
				WHEN 'Kamis' THEN 4
				WHEN 'Jumat' THEN 5
				WHEN 'Sabtu' THEN 6
				WHEN 'Minggu' THEN 7
			END,
			waktu_mulai ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil jadwal dari database: %w", err)
	}
	defer rows.Close()

	var listJadwal []models.Jadwal
	for rows.Next() {
		var j models.Jadwal
		if err := rows.Scan(&j.ID, &j.Hari, &j.WaktuMulai, &j.WaktuSelesai, &j.Ruang, &j.MatKul, &j.Dosen, &j.Semester, &j.SKS, &j.IsActive, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gagal membaca data jadwal: %w", err)
		}
		listJadwal = append(listJadwal, j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error setelah membaca jadwal: %w", err)
	}
	return listJadwal, nil
}

// GetByID mengambil jadwal berdasarkan ID
func (r *JadwalRepository) GetByID(ctx context.Context, id int) (*models.Jadwal, error) {
	query := `
		SELECT id, hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks, is_active, created_at, updated_at
		FROM jadwal_kelas
		WHERE id = $1
	`

	var j models.Jadwal
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&j.ID, &j.Hari, &j.WaktuMulai, &j.WaktuSelesai, &j.Ruang, &j.MatKul, &j.Dosen, &j.Semester, &j.SKS, &j.IsActive, &j.CreatedAt, &j.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("jadwal dengan ID %d tidak ditemukan", id)
	}
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil jadwal: %w", err)
	}

	return &j, nil
}

// GetByHari mengambil jadwal berdasarkan hari
func (r *JadwalRepository) GetByHari(ctx context.Context, hari string) ([]models.Jadwal, error) {
	query := `
		SELECT id, hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks, is_active, created_at, updated_at
		FROM jadwal_kelas
		WHERE hari = $1 AND is_active = true
		ORDER BY waktu_mulai ASC
	`

	rows, err := r.db.QueryContext(ctx, query, hari)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil jadwal untuk hari %s: %w", hari, err)
	}
	defer rows.Close()

	var listJadwal []models.Jadwal
	for rows.Next() {
		var j models.Jadwal
		if err := rows.Scan(&j.ID, &j.Hari, &j.WaktuMulai, &j.WaktuSelesai, &j.Ruang, &j.MatKul, &j.Dosen, &j.Semester, &j.SKS, &j.IsActive, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gagal membaca data jadwal: %w", err)
		}
		listJadwal = append(listJadwal, j)
	}

	return listJadwal, rows.Err()
}

// GetScheduleForTomorrow mengambil jadwal untuk besok (untuk reminder)
func (r *JadwalRepository) GetScheduleForTomorrow(ctx context.Context) ([]models.Jadwal, error) {
	tomorrow := time.Now().Add(24 * time.Hour)
	hariIndonesia := map[time.Weekday]string{
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
		time.Saturday:  "Sabtu",
		time.Sunday:    "Minggu",
	}

	hari := hariIndonesia[tomorrow.Weekday()]
	return r.GetByHari(ctx, hari)
}

// Create menambahkan jadwal baru
func (r *JadwalRepository) Create(ctx context.Context, j *models.Jadwal) error {
	query := `
		INSERT INTO jadwal_kelas (hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		j.Hari, j.WaktuMulai, j.WaktuSelesai, j.Ruang, j.MatKul, j.Dosen, j.Semester, j.SKS, j.IsActive,
	).Scan(&j.ID, &j.CreatedAt, &j.UpdatedAt)

	if err != nil {
		return fmt.Errorf("gagal menambahkan jadwal: %w", err)
	}

	return nil
}

// Update memperbarui jadwal yang ada
func (r *JadwalRepository) Update(ctx context.Context, j *models.Jadwal) error {
	query := `
		UPDATE jadwal_kelas
		SET hari = $1, waktu_mulai = $2, waktu_selesai = $3, ruang = $4, matkul = $5, dosen = $6, semester = $7, sks = $8, is_active = $9
		WHERE id = $10
	`

	result, err := r.db.ExecContext(ctx, query,
		j.Hari, j.WaktuMulai, j.WaktuSelesai, j.Ruang, j.MatKul, j.Dosen, j.Semester, j.SKS, j.IsActive, j.ID,
	)
	if err != nil {
		return fmt.Errorf("gagal memperbarui jadwal: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("gagal mengecek rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("jadwal dengan ID %d tidak ditemukan", j.ID)
	}

	return nil
}

// Delete menghapus jadwal (soft delete dengan set is_active = false)
func (r *JadwalRepository) Delete(ctx context.Context, id int) error {
	query := `UPDATE jadwal_kelas SET is_active = false WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus jadwal: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("gagal mengecek rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("jadwal dengan ID %d tidak ditemukan", id)
	}

	return nil
}

// HardDelete menghapus jadwal secara permanen dari database
func (r *JadwalRepository) HardDelete(ctx context.Context, id int) error {
	query := `DELETE FROM jadwal_kelas WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus jadwal secara permanen: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("gagal mengecek rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("jadwal dengan ID %d tidak ditemukan", id)
	}

	return nil
}
