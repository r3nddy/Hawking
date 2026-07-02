package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken  string
	DatabaseURL   string
	SpotifyID     string
	SpotifySecret string
	OwnerID       string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: .env file not found, relying on system environment variables")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN not found in environment variables")
	}

	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		dbUrl = os.Getenv("SUPABASE_DB_URL")
	}
	if dbUrl == "" {
		log.Fatal("DATABASE_URL or SUPABASE_DB_URL not found in environment variables")
	}

	spotifyID := os.Getenv("SPOTIFY_CLIENT_ID")
	if spotifyID == "" {
		spotifyID = os.Getenv("SPOTIFY_ID")
	}

	spotifySecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if spotifySecret == "" {
		spotifySecret = os.Getenv("SPOTIFY_SECRET")
	}

	ownerID := os.Getenv("OWNER_ID")
	if ownerID == "" {
		log.Println("Warning: OWNER_ID is not set in environment variables. Bot owner commands will not work.")
	}

	return &Config{
		DiscordToken:  token,
		DatabaseURL:   dbUrl,
		SpotifyID:     spotifyID,
		SpotifySecret: spotifySecret,
		OwnerID:       ownerID,
	}
}
