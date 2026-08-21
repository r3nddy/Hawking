package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations menjalankan semua migration files secara otomatis dari folder migrations/
func RunMigrations(db *sql.DB, migrationsPath string) error {
	// Setup driver postgres
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create postgres driver: %w", err)
	}

	// Resolve absolute path untuk migrations
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("could not resolve migrations path: %w", err)
	}

	// Cek apakah direktori migrations ada
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory does not exist: %s", absPath)
	}

	log.Printf("Loading migrations from: %s", absPath)

	// Buat migrate instance dari folder migrations dengan absolute path
	sourceURL := fmt.Sprintf("file://%s", filepath.ToSlash(absPath))
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	// Jalankan migration ke versi terbaru
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Printf("Warning: could not get migration version: %v", err)
	} else if err == nil {
		log.Printf("✓ Database migration completed. Current version: %d (dirty: %v)", version, dirty)
	} else {
		log.Println("✓ Database schema is up to date (no migrations applied yet)")
	}

	return nil
}
