package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"hawking-bot/internal/config"
	"hawking-bot/internal/discord"
	"hawking-bot/internal/handlers"
	"hawking-bot/internal/repository"
	"hawking-bot/internal/services"
)

func Run() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error opening database connection: ", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Error pinging database: ", err)
	}

	// Repositories
	authRepo := repository.NewAuthRepository(db)
	jadwalRepo := repository.NewJadwalRepository(db)
	storageRepo := repository.NewStorageRepository(db)

	// Services
	authSvc := services.NewAuthService(authRepo, cfg.OwnerID)
	jadwalSvc := services.NewJadwalService(jadwalRepo)
	spotifySvc := services.NewSpotifyService(cfg.SpotifyID, cfg.SpotifySecret)

	// Discord Router & Client
	router := discord.NewRouter(cfg.GuildID)
	
	client, err := discord.NewClient(cfg.DiscordToken, router)
	if err != nil {
		log.Fatal("Error creating discord client: ", err)
	}
	defer client.Close()

	botUser, err := client.Session.User("@me")
	if err != nil {
		log.Fatal("Error fetching bot user info: ", err)
	}

	musicSvc, err := services.NewMusicService(botUser.ID, spotifySvc)
	if err != nil {
		log.Fatal("Error creating music service: ", err)
	}

	// Handlers
	handlers.NewAuthHandler(authSvc, router)
	handlers.NewJadwalHandler(jadwalSvc, router)
	handlers.NewMusicHandler(musicSvc, authSvc, spotifySvc, storageRepo, router)

	// Connect Client (Registers commands and opens websocket)
	if err := client.Connect(musicSvc); err != nil {
		log.Fatal("Error connecting discord client: ", err)
	}

	// Connect Music Service
	if err := musicSvc.Connect(context.Background()); err != nil {
		log.Fatal("Error connecting lavalink: ", err)
	}

	fmt.Println("Bot is Running! (Modular Monolith)")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down...")
}
