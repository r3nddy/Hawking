package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"

	"hawking-bot/internal/config"
	"hawking-bot/internal/database"
	"hawking-bot/internal/discord"
	"hawking-bot/internal/handlers"
	"hawking-bot/internal/repository"
	"hawking-bot/internal/scheduler"
	"hawking-bot/internal/services"
)

func Run() {
	cfg := config.Load()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Error opening database connection: ", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Error pinging database: ", err)
	}

	log.Println("Running database migrations...")
	if err := database.RunMigrations(db, "migrations"); err != nil {
		log.Fatal("Error running migrations: ", err)
	}

	// Repositories
	authRepo := repository.NewAuthRepository(db)
	jadwalRepo := repository.NewJadwalRepository(db)
	reminderRepo := repository.NewReminderRepository(db)
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

	// Reminder Service (needs Discord session)
	reminderSvc := services.NewReminderService(jadwalRepo, reminderRepo, client.Session)

	// Handlers
	handlers.NewAuthHandler(authSvc, router)
	handlers.NewJadwalHandler(jadwalSvc, router)
	handlers.NewReminderHandler(reminderSvc, authSvc, router)
	handlers.NewMusicHandler(musicSvc, authSvc, spotifySvc, storageRepo, router)

	// Connect Client (Registers commands and opens websocket)
	if err := client.Connect(musicSvc); err != nil {
		log.Fatal("Error connecting discord client: ", err)
	}

	// Connect Music Service
	if err := musicSvc.Connect(context.Background()); err != nil {
		log.Fatal("Error connecting lavalink: ", err)
	}

	// Start Reminder Scheduler
	reminderScheduler := scheduler.NewReminderScheduler(reminderSvc)
	if err := reminderScheduler.Start(); err != nil {
		log.Fatal("Error starting reminder scheduler: ", err)
	}
	defer reminderScheduler.Stop()

	fmt.Println("Bot is Running! (Modular Monolith)")
	fmt.Printf("Next reminder check: %s\n", reminderScheduler.GetNextRun().Format("2006-01-02 15:04:05"))

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down...")
	log.Println("Stopping reminder scheduler...")
	reminderScheduler.Stop()
	log.Println("Shutdown complete")
}
