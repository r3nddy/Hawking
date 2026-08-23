-- Rollback: Remove seed data jadwal kuliah

-- Hapus semua data jadwal yang di-insert di seed
DELETE FROM jadwal_kelas WHERE semester IN ('2025 B', '2025 AB', '2025 BC');
