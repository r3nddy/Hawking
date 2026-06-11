package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"database/sql"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: .env file not found, relying on system environment variables")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN not found in .env file")
	}

	dbUrl := os.Getenv("SUPABASE_DB_URL")
	if dbUrl == "" {
		log.Fatal("SUPABASE_DB_URL not found in .env file")
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatal("Error opening database connection: ", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Error pinging database: ", err)
	}

	ownerID := os.Getenv("OWNER_ID")
	if ownerID == "" {
		log.Println("Warning: OWNER_ID is not set in .env. Bot owner commands will not work.")
	}
	authManager := NewAuthManager(ownerID, db)
	storageManager := NewStorageManager(db)

	spotifyManager := NewSpotifyManager()

	bot, err := NewBot(token, db, authManager, spotifyManager, storageManager)
	if err != nil {
		log.Fatal("Error creating bot: ", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatal("Error starting bot: ", err)
	}

	defer bot.Stop()

	fmt.Println("Bot is Running!")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	
}