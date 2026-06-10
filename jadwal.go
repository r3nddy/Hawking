package main

import (
	"database/sql"
	"fmt"
)

type Jadwal struct {
	Hari     string
	Waktu    string
	Ruang    string
	MatKul   string
	Dosen    string
	Semester string
	SKS      string
}

func GetJadwal(db *sql.DB) string {
	rows, err := db.Query("SELECT hari, waktu, ruang, matkul, dosen, semester, sks FROM jadwal ORDER BY id ASC")
	if err != nil {
		return fmt.Sprintf("Gagal mengambil jadwal dari database: %v", err)
	}
	defer rows.Close()

	var listJadwal []Jadwal
	for rows.Next() {
		var j Jadwal
		if err := rows.Scan(&j.Hari, &j.Waktu, &j.Ruang, &j.MatKul, &j.Dosen, &j.Semester, &j.SKS); err != nil {
			return fmt.Sprintf("Gagal membaca data jadwal: %v", err)
		}
		listJadwal = append(listJadwal, j)
	}

	if err := rows.Err(); err != nil {
		return fmt.Sprintf("Error setelah membaca jadwal: %v", err)
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