package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/snowflake/v2"
)

type Bot struct {
	session *discordgo.Session
	db      *sql.DB
	auth    *AuthManager
	spotify *SpotifyManager
	storage *StorageManager
	music   *MusicManager
}

func NewBot(token string, db *sql.DB, auth *AuthManager, spotify *SpotifyManager, storage *StorageManager) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuilds

	return &Bot{session: dg, db: db, auth: auth, spotify: spotify, storage: storage}, nil
}

func (b *Bot) Start() error {
	b.registerHandlers()

	if err := b.session.Open(); err != nil {
		return err
	}

	music, err := NewMusicManager(b.session.State.User.ID, b.spotify)
	if err != nil {
		return fmt.Errorf("music manager: %w", err)
	}
	if err := music.Connect(context.Background()); err != nil {
		return fmt.Errorf("lavalink connect: %w", err)
	}
	b.music = music

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
	b.session.AddHandler(b.handleVoiceStateUpdate)
	b.session.AddHandler(b.handleVoiceServerUpdate)
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
		{
			Name:        "join",
			Description: "Membuat bot masuk ke voice channel Anda",
		},
		{
			Name:        "leave",
			Description: "Membuat bot keluar dari voice channel",
		},
		{
			Name:        "play",
			Description: "Cari dan putar lagu (YouTube/Spotify URL atau kata kunci)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "query",
					Description: "URL atau kata kunci pencarian",
					Required:    true,
				},
			},
		},
		{
			Name:        "pause",
			Description: "Jeda lagu yang sedang diputar",
		},
		{
			Name:        "resume",
			Description: "Lanjutkan lagu yang dijeda",
		},
		{
			Name:        "skip",
			Description: "Lewati lagu saat ini",
		},
		{
			Name:        "stop",
			Description: "Hentikan pemutaran dan kosongkan antrian",
		},
		{
			Name:        "queue",
			Description: "Tampilkan antrian lagu",
		},
		{
			Name:        "nowplaying",
			Description: "Tampilkan lagu yang sedang diputar",
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

	case "join":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		vs, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda harus berada di voice channel terlebih dahulu agar bot bisa bergabung."},
			})
			return
		}

		err = s.ChannelVoiceJoinManual(i.GuildID, vs.ChannelID, false, false)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Gagal bergabung ke voice channel: " + err.Error()},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("🔊 Berhasil bergabung ke voice channel <#%s>!", vs.ChannelID)},
		})

	case "leave":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		botVS, err := s.State.VoiceState(i.GuildID, s.State.User.ID)
		if err != nil || botVS.ChannelID == "" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Bot tidak sedang berada di voice channel mana pun."},
			})
			return
		}

		if b.music != nil {
			_ = b.music.Stop(context.Background(), i.GuildID)
		}

		err = s.ChannelVoiceJoinManual(i.GuildID, "", false, false)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Gagal keluar dari voice channel: " + err.Error()},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "👋 Berhasil keluar dari voice channel!"},
		})

	case "play":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		if b.music == nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Sistem musik belum siap."},
			})
			return
		}

		vs, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda harus berada di voice channel terlebih dahulu."},
			})
			return
		}

		botVS, _ := s.State.VoiceState(i.GuildID, s.State.User.ID)
		needsJoin := botVS == nil || botVS.ChannelID == "" || botVS.ChannelID != vs.ChannelID
		if needsJoin {
			err = s.ChannelVoiceJoinManual(i.GuildID, vs.ChannelID, false, false)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{Content: "❌ Gagal bergabung ke voice channel: " + err.Error()},
				})
				return
			}
			time.Sleep(2 * time.Second)
		}

		query := i.ApplicationCommandData().Options[0].StringValue()

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		result, err := b.music.Play(context.Background(), i.GuildID, query)
		if err != nil {
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: stringPtr("❌ Gagal memutar lagu: " + err.Error()),
			})
			return
		}

		var msg string
		if result.Action == "queued" {
			msg = fmt.Sprintf("➕ Ditambahkan ke antrian: %s", formatTrack(result.Track))
		} else {
			msg = fmt.Sprintf("🎵 Now playing: %s", formatTrack(result.Track))
		}
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: stringPtr(msg),
		})

	case "pause":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		if err := b.music.Pause(context.Background(), i.GuildID); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ " + err.Error()},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "⏸️ Musik dijeda."},
		})

	case "resume":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		if err := b.music.Resume(context.Background(), i.GuildID); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ " + err.Error()},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "▶️ Musik dilanjutkan."},
		})

	case "skip":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		next, err := b.music.Skip(context.Background(), i.GuildID)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ " + err.Error()},
			})
			return
		}

		if next == nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "⏭️ Lagu dilewati. Antrian kosong."},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("⏭️ Now playing: %s", formatTrack(*next))},
		})

	case "stop":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		if err := b.music.Stop(context.Background(), i.GuildID); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ " + err.Error()},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "⏹️ Pemutaran dihentikan dan antrian dikosongkan."},
		})

	case "queue":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		queue := b.music.GetQueue(i.GuildID)
		nowPlaying, _, _ := b.music.GetNowPlaying(i.GuildID)

		if nowPlaying == nil && len(queue) == 0 {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "📭 Antrian kosong."},
			})
			return
		}

		var msg strings.Builder
		msg.WriteString("**Antrian Lagu:**\n")
		if nowPlaying != nil {
			msg.WriteString(fmt.Sprintf("🎵 **Sedang diputar:** %s\n", formatTrack(*nowPlaying)))
		}
		for idx, t := range queue {
			msg.WriteString(fmt.Sprintf("%d. %s\n", idx+1, formatTrack(t)))
			if idx >= 9 {
				msg.WriteString("...dan lainnya.\n")
				break
			}
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: msg.String()},
		})

	case "nowplaying":
		if !b.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Anda tidak memiliki izin."},
			})
			return
		}

		track, paused, position := b.music.GetNowPlaying(i.GuildID)
		if track == nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "❌ Tidak ada lagu yang sedang diputar."},
			})
			return
		}

		status := "▶️ Playing"
		if paused {
			status = "⏸️ Paused"
		}

		msg := fmt.Sprintf("%s: %s\n`%s / %s`",
			status,
			formatTrack(*track),
			formatDuration(position),
			formatDuration(track.Info.Length),
		)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: msg},
		})

	default:
		fmt.Println("Unknown command:", i.ApplicationCommandData().Name)
	}
}

func (b *Bot) handleVoiceStateUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	if b.music == nil || e.UserID != s.State.User.ID {
		return
	}

	guildID, err := snowflake.Parse(e.GuildID)
	if err != nil {
		return
	}

	var channelID *snowflake.ID
	if e.ChannelID != "" {
		id, err := snowflake.Parse(e.ChannelID)
		if err != nil {
			return
		}
		channelID = &id
	}

	b.music.Client().OnVoiceStateUpdate(context.Background(), guildID, channelID, e.SessionID)
}

func (b *Bot) handleVoiceServerUpdate(s *discordgo.Session, e *discordgo.VoiceServerUpdate) {
	if b.music == nil {
		return
	}

	guildID, err := snowflake.Parse(e.GuildID)
	if err != nil {
		return
	}

	if e.Endpoint == "" || e.Token == "" {
		return
	}

	endpoint := normalizeVoiceEndpoint(e.Endpoint)
	b.music.Client().OnVoiceServerUpdate(context.Background(), guildID, e.Token, endpoint)
}

func normalizeVoiceEndpoint(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "wss://")
	endpoint = strings.TrimPrefix(endpoint, "ws://")
	return endpoint
}

func stringPtr(s string) *string {
	return &s
}