-- Migration: Seed data jadwal kuliah kelas 2025 B
-- Created: 2026-08-21

-- Hapus data lama jika ada (opsional, uncomment jika ingin reset)
-- DELETE FROM jadwal_kelas;

-- SENIN
INSERT INTO jadwal_kelas (hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks)
VALUES
('Senin', '13:00', '14:30', 'C306', 'Analisis Kompleksitas Algoritma', 'Medi Taruk, S.Kom., M.Cs / Masna Wati, S.Si., M.T', '2025 AB', 3);

-- SELASA
INSERT INTO jadwal_kelas (hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks)
VALUES
('Selasa', '07:30', '09:00', 'C302', 'Dasar Kecerdasan Artifisial', 'Ir. Indah Fitri Astuti, S.Kom., M.Cs / Ir. Addy Suyatno, S.Kom., M.Kom', '2025 B', 3),
('Selasa', '10:50', '12:20', 'C307', 'Sistem Operasi', 'Ir. Dedy Cahyadi, S.Kom., M.Eng / Gubtha Mahendra Putra, S.Kom., M.Eng', '2025 B', 3),
('Selasa', '14:40', '16:10', 'C302', 'Wirausaha Teknologi', 'Rasni Alex, S.E., M.M / Dr. Ir. Nataniel Dengen, S.Si., M.Si', '2025 B', 3);

-- RABU
INSERT INTO jadwal_kelas (hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks)
VALUES
('Rabu', '07:30', '09:00', 'C304', 'Jaringan Komputer', 'Ir. Dedy Cahyadi, S.Kom., M.Eng / Reza Wardhana, S.Kom., M.Eng', '2025 B', 3),
('Rabu', '09:10', '10:40', 'C304', 'Matematika Diskrit', 'Prof. Dr. Fahrul Agus, S.Si., M.T / Prof. Dr. Ir. Anindita Septiarini, ST., M.Cs., IPU', '2025 BC', 3);

-- KAMIS
INSERT INTO jadwal_kelas (hari, waktu_mulai, waktu_selesai, ruang, matkul, dosen, semester, sks)
VALUES
('Kamis', '07:30', '09:00', 'C306', 'Pemrograman Berorientasi Objek', 'Anton Prafanto, S.Kom., M.T / Rajiansyah, S.Kom., M.Sc', '2025 AB', 3),
('Kamis', '09:10', '10:40', 'C306', 'Rekayasa Perangkat Lunak', 'Ramadiani, S.Pd., M.Si., M.Kom., Ph.D / Ummul Hairah, S.Pd., M.T', '2025 AB', 3);

-- Verifikasi data
SELECT hari, waktu_mulai, waktu_selesai, matkul, ruang, dosen
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
    waktu_mulai ASC;
