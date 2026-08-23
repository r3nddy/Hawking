package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Get DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("SUPABASE_DB_URL")
	}
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL or SUPABASE_DB_URL not found in environment variables")
	}

	// Mask password for display
	maskedURL := maskPassword(dbURL)
	fmt.Printf("🔗 Attempting to connect to: %s\n", maskedURL)
	fmt.Println()

	// Open connection
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("❌ Error opening database connection: %v", err)
	}
	defer db.Close()

	fmt.Println("✅ Database connection opened successfully")

	// Ping database
	fmt.Println("🔍 Pinging database...")
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Error pinging database: %v\n\nPossible causes:\n  - Invalid connection string\n  - Database host unreachable\n  - Authentication failed\n  - SSL/TLS misconfiguration\n", err)
	}

	fmt.Println("✅ Database ping successful!")
	fmt.Println()

	// Get database version
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("❌ Error querying database version: %v", err)
	}
	fmt.Printf("📊 Database version: %s\n", version)
	fmt.Println()

	// Check if schema_migrations table exists
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'schema_migrations'
		)
	`).Scan(&exists)
	if err != nil {
		log.Printf("⚠️  Warning: Could not check schema_migrations table: %v", err)
	} else if exists {
		var currentVersion int
		var dirty bool
		err = db.QueryRow("SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&currentVersion, &dirty)
		if err != nil {
			log.Printf("⚠️  Warning: Could not read schema_migrations: %v", err)
		} else {
			fmt.Printf("📋 Current migration version: %d (dirty: %v)\n", currentVersion, dirty)
		}
	} else {
		fmt.Println("📋 No migrations have been run yet (schema_migrations table not found)")
	}
	fmt.Println()

	// List all tables
	rows, err := db.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name
	`)
	if err != nil {
		log.Printf("⚠️  Warning: Could not list tables: %v", err)
	} else {
		defer rows.Close()

		tables := []string{}
		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				continue
			}
			tables = append(tables, tableName)
		}

		if len(tables) > 0 {
			fmt.Printf("📚 Tables in database (%d):\n", len(tables))
			for _, table := range tables {
				fmt.Printf("   - %s\n", table)
			}
		} else {
			fmt.Println("📚 No tables found in database")
		}
	}
	fmt.Println()

	// Check jadwal_kelas if exists
	var jadwalExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'jadwal_kelas'
		)
	`).Scan(&jadwalExists)
	if err == nil && jadwalExists {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM jadwal_kelas").Scan(&count)
		if err != nil {
			log.Printf("⚠️  Warning: Could not count jadwal_kelas records: %v", err)
		} else {
			fmt.Printf("📅 Total jadwal_kelas records: %d\n", count)
			if count == 0 {
				fmt.Println("   ⚠️  No jadwal records found. Migration 002_seed_jadwal.up.sql may not have run.")
			} else if count == 8 {
				fmt.Println("   ✅ All jadwal records loaded successfully!")
			}
		}
	}

	fmt.Println()
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")
	fmt.Println("✅ Connection test completed successfully!")
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")
}

// maskPassword replaces password in connection string with asterisks
func maskPassword(connStr string) string {
	// Simple masking: find password part between :// and @
	start := -1
	for i := 0; i < len(connStr)-2; i++ {
		if connStr[i:i+3] == "://" {
			start = i + 3
			break
		}
	}
	if start == -1 {
		return connStr
	}

	colonPos := -1
	atPos := -1
	for i := start; i < len(connStr); i++ {
		if connStr[i] == ':' && colonPos == -1 {
			colonPos = i
		}
		if connStr[i] == '@' {
			atPos = i
			break
		}
	}

	if colonPos == -1 || atPos == -1 || colonPos >= atPos {
		return connStr
	}

	// Replace password with asterisks
	return connStr[:colonPos+1] + "********" + connStr[atPos:]
}
