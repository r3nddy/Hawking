package main

import (
	"fmt"
	"log"
	"database/sql"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
	db      *sql.DB
}

func NewBot(token string, db *sql.DB) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	return &Bot{session: dg, db: db}, nil
}

func (b *Bot) Start() error {
	b.registerHandlers()

	if err := b.session.Open(); err != nil {
		return err
	}

	if err := b.registerCommands(); err != nil {
		return err
	}

	return nil
}

func (b *Bot) Stop() {
	b.session.Close()
}

func (b *Bot) registerHandlers() {
	b.session.AddHandler(b.handleMessage)
	b.session.AddHandler(b.handleSlashCommand)
}

func (b *Bot) registerCommands() error {
	_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "jadwal",
		Description: "Tampilkan jadwal kuliah kelas B 25",
	})
	if err != nil {
		return err
	}

	log.Println("Slash command /jadwal berhasil didaftarkan")
	return nil
}

func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "ping" {
		s.ChannelMessageSend(m.ChannelID, "pong!")
	}
}

func (b *Bot) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.ApplicationCommandData().Name {
	case "jadwal":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: GetJadwal(b.db),
			},
		})
	default:
		fmt.Println("Unknown command:", i.ApplicationCommandData().Name)
	}
}