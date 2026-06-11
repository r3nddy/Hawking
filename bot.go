package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
	db      *sql.DB
	auth    *AuthManager
	spotify *SpotifyManager
	storage *StorageManager
}

func NewBot(token string, db *sql.DB, auth *AuthManager, spotify *SpotifyManager, storage *StorageManager) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	return &Bot{session: dg, db: db, auth: auth, spotify: spotify, storage: storage}, nil
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
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "jadwal",
			Description: "Tampilkan jadwal kuliah kelas B 25",
		},
		{
			Name:        "grant",
			Description: "Memberikan izin menggunakan command musik ke user lain (Khusus Owner)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User yang akan diberi izin",
					Required:    true,
				},
			},
		},
		{
			Name:        "revoke",
			Description: "Mencabut izin menggunakan command musik dari user lain (Khusus Owner)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User yang akan dicabut izinnya",
					Required:    true,
				},
			},
		},
		{
			Name:        "save",
			Description: "Menyimpan lagu/playlist dari Spotify (Khusus Owner & Authorized)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "URL lagu Spotify",
					Required:    true,
				},
			},
		},
		{
			Name:        "mysongs",
			Description: "Melihat daftar lagu yang sudah disimpan",
		},
	}

	for _, v := range commands {
		_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", v)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", v.Name, err)
		}
	}

	log.Println("Slash commands berhasil didaftarkan")
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
	case "grant":
		if !b.auth.IsOwner(i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Hanya Owner bot yang dapat menggunakan command ini."},
			})
			return
		}
		
		targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
		err := b.auth.GrantAccess(context.Background(), targetUser.ID, i.Member.User.ID)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Gagal memberikan izin ke database."},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Berhasil memberikan izin akses kepada <@%s>", targetUser.ID)},
		})

	case "revoke":
		if !b.auth.IsOwner(i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Hanya Owner bot yang dapat menggunakan command ini."},
			})
			return
		}

		targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
		err := b.auth.RevokeAccess(context.Background(), targetUser.ID)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Gagal mencabut izin dari database."},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Berhasil mencabut izin akses dari <@%s>", targetUser.ID)},
		})

	case "save":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin untuk menyimpan musik."},
			})
			return
		}

		if b.spotify == nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Spotify API belum dikonfigurasi."},
			})
			return
		}

		url := i.ApplicationCommandData().Options[0].StringValue()
		trackID := ExtractTrackID(url)
		if trackID == "" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ URL Spotify tidak valid."},
			})
			return
		}

		// Karena API call Spotify lambat, respon awal dengan "Deferred"
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		title, artist, err := b.spotify.GetTrackInfo(context.Background(), trackID)
		if err != nil {
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: stringPtr("❌ Gagal mendapatkan detail lagu dari Spotify: " + err.Error()),
			})
			return
		}

		err = b.storage.SaveTrack(context.Background(), i.Member.User.ID, title, artist, url)
		if err != nil {
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: stringPtr("❌ Gagal menyimpan lagu ke database."),
			})
			return
		}

		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: stringPtr(fmt.Sprintf("🎵 Berhasil menyimpan **%s** oleh **%s**!", title, artist)),
		})

	case "mysongs":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		tracks, err := b.storage.GetTracks(context.Background(), i.Member.User.ID)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Gagal mengambil lagu dari database."},
			})
			return
		}

		if len(tracks) == 0 {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "📭 Anda belum menyimpan lagu apa pun."},
			})
			return
		}

		msg := "**Playlist Tersimpan:**\n"
		for idx, t := range tracks {
			msg += fmt.Sprintf("%d. **%s** - %s\n", idx+1, t.TrackTitle, t.TrackArtist)
			if idx >= 9 { // limit 10
				msg += "...dan lainnya.\n"
				break
			}
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: msg},
		})

	default:
		fmt.Println("Unknown command:", i.ApplicationCommandData().Name)
	}
}

func stringPtr(s string) *string {
	return &s
}