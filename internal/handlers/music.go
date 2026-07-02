package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/v3/lavalink"
	"hawking-bot/internal/discord"
	"hawking-bot/internal/repository"
	"hawking-bot/internal/services"
)

type MusicHandler struct {
	music   *services.MusicService
	auth    *services.AuthService
	spotify *services.SpotifyService
	storage *repository.StorageRepository
}

func NewMusicHandler(music *services.MusicService, auth *services.AuthService, spotify *services.SpotifyService, storage *repository.StorageRepository, router *discord.Router) *MusicHandler {
	h := &MusicHandler{
		music:   music,
		auth:    auth,
		spotify: spotify,
		storage: storage,
	}

	router.Register(&discordgo.ApplicationCommand{
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
	}, h.HandleSave)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "mysongs",
		Description: "Kelola lagu yang sudah disimpan",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mode",
				Description: "Pilih aksi untuk playlist tersimpan",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "list", Value: "list"},
					{Name: "play", Value: "play"},
					{Name: "stopcycle", Value: "stopcycle"},
					{Name: "delete", Value: "delete"},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "id",
				Description: "ID lagu tersimpan yang akan dihapus",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "page",
				Description: "Nomor halaman playlist tersimpan",
				Required:    false,
			},
		},
	}, h.HandleMySongs)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "join",
		Description: "Membuat bot masuk ke voice channel Anda",
	}, h.HandleJoin)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "leave",
		Description: "Membuat bot keluar dari voice channel",
	}, h.HandleLeave)

	router.Register(&discordgo.ApplicationCommand{
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
	}, h.HandlePlay)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "pause",
		Description: "Jeda lagu yang sedang diputar",
	}, h.HandlePause)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "resume",
		Description: "Lanjutkan lagu yang dijeda",
	}, h.HandleResume)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "skip",
		Description: "Lewati lagu saat ini",
	}, h.HandleSkip)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "stop",
		Description: "Hentikan pemutaran dan kosongkan antrian",
	}, h.HandleStop)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "queue",
		Description: "Tampilkan antrian lagu",
	}, h.HandleQueue)

	router.Register(&discordgo.ApplicationCommand{
		Name:        "nowplaying",
		Description: "Tampilkan lagu yang sedang diputar",
	}, h.HandleNowPlaying)

	return h
}

func (h *MusicHandler) checkAuth(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	if !h.auth.IsAuthorized(context.Background(), i.Member.User.ID) {
		respond(s, i, "❌ Anda tidak memiliki izin.")
		return false
	}
	return true
}

func (h *MusicHandler) HandleSave(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	if h.spotify == nil {
		respond(s, i, "❌ Spotify API belum dikonfigurasi.")
		return
	}

	url := i.ApplicationCommandData().Options[0].StringValue()
	trackID := services.ExtractTrackID(url)
	if trackID == "" {
		respond(s, i, "❌ URL Spotify tidak valid.")
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	title, artist, err := h.spotify.GetTrackInfo(context.Background(), trackID)
	if err != nil {
		msg := "❌ Gagal mendapatkan detail lagu dari Spotify: " + err.Error()
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}

	err = h.storage.SaveTrack(context.Background(), i.Member.User.ID, title, artist, url)
	if err != nil {
		msg := "❌ Gagal menyimpan lagu ke database."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}

	msg := fmt.Sprintf("🎵 Berhasil menyimpan **%s** oleh **%s**!", title, artist)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
}

func (h *MusicHandler) HandleMySongs(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	mode := "list"
	var deleteID int
	hasDeleteID := false
	for _, option := range i.ApplicationCommandData().Options {
		switch option.Name {
		case "mode":
			mode = option.StringValue()
		case "id":
			deleteID = int(option.IntValue())
			hasDeleteID = true
		}
	}

	if mode == "stopcycle" {
		if h.music != nil {
			h.music.StopCycle(i.GuildID)
		}
		respond(s, i, "Cycle playlist tersimpan dimatikan.")
		return
	}

	if mode == "delete" {
		if !hasDeleteID || deleteID <= 0 {
			respond(s, i, "Masukkan ID lagu yang valid. Contoh: /mysongs mode:delete id:12")
			return
		}

		deleted, err := h.storage.DeleteTrack(context.Background(), i.Member.User.ID, deleteID)
		if err != nil {
			respond(s, i, "Gagal menghapus lagu dari Supabase.")
			return
		}
		if !deleted {
			respond(s, i, "ID lagu tidak ditemukan di playlist kamu.")
			return
		}

		respond(s, i, fmt.Sprintf("Lagu dengan ID %d berhasil dihapus dari playlist tersimpan.", deleteID))
		return
	}

	if mode == "list" {
		// Pagination omitted for brevity in this refactor step, returning all
		tracks, err := h.storage.GetTracks(context.Background(), i.Member.User.ID)
		if err != nil {
			respond(s, i, "Gagal mengambil playlist dari Supabase.")
			return
		}

		var msg strings.Builder
		msg.WriteString("**Playlist Tersimpan:**\n")
		for idx, t := range tracks {
			msg.WriteString(fmt.Sprintf("ID %d - **%s** - %s\n", t.ID, t.TrackTitle, t.TrackArtist))
			if idx >= 9 { // limit 10
				msg.WriteString("...dan lainnya.\n")
				break
			}
		}
		if len(tracks) == 0 {
			msg.WriteString("Belum ada lagu.")
		}
		respond(s, i, msg.String())
		return
	}

	tracks, err := h.storage.GetTracks(context.Background(), i.Member.User.ID)
	if err != nil {
		respond(s, i, "❌ Gagal mengambil lagu dari database.")
		return
	}

	if len(tracks) == 0 {
		respond(s, i, "📭 Anda belum menyimpan lagu apa pun.")
		return
	}

	if mode == "play" {
		if h.music == nil {
			respond(s, i, "Sistem musik belum siap.")
			return
		}

		vs, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
		if err != nil {
			respond(s, i, "Anda harus berada di voice channel terlebih dahulu.")
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		h.music.ResetVoiceGate(i.GuildID)

		botVS, _ := s.State.VoiceState(i.GuildID, s.State.User.ID)
		needsJoin := botVS == nil || botVS.ChannelID == "" || botVS.ChannelID != vs.ChannelID
		if needsJoin {
			err = s.ChannelVoiceJoinManual(i.GuildID, vs.ChannelID, false, false)
			if err != nil {
				msg := "Gagal bergabung ke voice channel: " + err.Error()
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
				return
			}
		}

		result, count, err := h.music.StartCycle(context.Background(), i.GuildID, tracks)
		if err != nil {
			msg := "Gagal memutar playlist tersimpan: " + err.Error()
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
			return
		}

		msg := fmt.Sprintf("Memutar %d lagu tersimpan secara cycle. Now playing: %s", count, formatTrackInfo(result.Track))
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}
}

func (h *MusicHandler) HandleJoin(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	vs, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		respond(s, i, "❌ Anda harus berada di voice channel terlebih dahulu agar bot bisa bergabung.")
		return
	}

	err = s.ChannelVoiceJoinManual(i.GuildID, vs.ChannelID, false, false)
	if err != nil {
		respond(s, i, "❌ Gagal bergabung ke voice channel: "+err.Error())
		return
	}

	respond(s, i, fmt.Sprintf("🔊 Berhasil bergabung ke voice channel <#%s>!", vs.ChannelID))
}

func (h *MusicHandler) HandleLeave(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	botVS, err := s.State.VoiceState(i.GuildID, s.State.User.ID)
	if err != nil || botVS.ChannelID == "" {
		respond(s, i, "❌ Bot tidak sedang berada di voice channel mana pun.")
		return
	}

	if h.music != nil {
		_ = h.music.Stop(context.Background(), i.GuildID)
	}

	err = s.ChannelVoiceJoinManual(i.GuildID, "", false, false)
	if err != nil {
		respond(s, i, "❌ Gagal keluar dari voice channel: "+err.Error())
		return
	}

	respond(s, i, "👋 Berhasil keluar dari voice channel!")
}

func (h *MusicHandler) HandlePlay(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	if h.music == nil {
		respond(s, i, "❌ Sistem musik belum siap.")
		return
	}

	vs, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		respond(s, i, "❌ Anda harus berada di voice channel terlebih dahulu.")
		return
	}

	h.music.ResetVoiceGate(i.GuildID)

	botVS, _ := s.State.VoiceState(i.GuildID, s.State.User.ID)
	needsJoin := botVS == nil || botVS.ChannelID == "" || botVS.ChannelID != vs.ChannelID
	if needsJoin {
		err = s.ChannelVoiceJoinManual(i.GuildID, vs.ChannelID, false, false)
		if err != nil {
			respond(s, i, "❌ Gagal bergabung ke voice channel: "+err.Error())
			return
		}
	}

	query := i.ApplicationCommandData().Options[0].StringValue()

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	result, err := h.music.Play(context.Background(), i.GuildID, query)
	if err != nil {
		msg := "❌ Gagal memutar lagu: " + err.Error()
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		return
	}

	var msg string
	if result.Action == "queued" {
		msg = fmt.Sprintf("➕ Ditambahkan ke antrian: %s", formatTrackInfo(result.Track))
	} else {
		msg = fmt.Sprintf("🎵 Now playing: %s", formatTrackInfo(result.Track))
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
}

func (h *MusicHandler) HandlePause(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}
	if err := h.music.Pause(context.Background(), i.GuildID); err != nil {
		respond(s, i, "❌ "+err.Error())
		return
	}
	respond(s, i, "⏸️ Musik dijeda.")
}

func (h *MusicHandler) HandleResume(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}
	if err := h.music.Resume(context.Background(), i.GuildID); err != nil {
		respond(s, i, "❌ "+err.Error())
		return
	}
	respond(s, i, "▶️ Musik dilanjutkan.")
}

func (h *MusicHandler) HandleSkip(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}
	next, err := h.music.Skip(context.Background(), i.GuildID)
	if err != nil {
		respond(s, i, "❌ "+err.Error())
		return
	}
	if next == nil {
		respond(s, i, "⏭️ Lagu dilewati. Antrian kosong.")
		return
	}
	respond(s, i, fmt.Sprintf("⏭️ Now playing: %s", formatTrackInfo(*next)))
}

func (h *MusicHandler) HandleStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}
	if err := h.music.Stop(context.Background(), i.GuildID); err != nil {
		respond(s, i, "❌ "+err.Error())
		return
	}
	respond(s, i, "⏹️ Pemutaran dihentikan dan antrian dikosongkan.")
}

func (h *MusicHandler) HandleQueue(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	queue := h.music.GetQueue(i.GuildID)
	nowPlaying, _, _ := h.music.GetNowPlaying(i.GuildID)
	cycle := h.music.GetCycleStatus(i.GuildID)

	if nowPlaying == nil && len(queue) == 0 && !cycle.Enabled {
		respond(s, i, "📭 Antrian kosong.")
		return
	}

	var msg strings.Builder
	msg.WriteString("**Antrian Lagu:**\n")
	if nowPlaying != nil {
		msg.WriteString(fmt.Sprintf("🎵 **Sedang diputar:** %s\n", formatTrackInfo(*nowPlaying)))
	}
	if cycle.Enabled {
		msg.WriteString(fmt.Sprintf("**Cycle aktif:** %d lagu tersimpan\n", cycle.Count))
	}
	for idx, t := range queue {
		msg.WriteString(fmt.Sprintf("%d. %s\n", idx+1, formatTrackInfo(t)))
		if idx >= 9 {
			msg.WriteString("...dan lainnya.\n")
			break
		}
	}

	respond(s, i, msg.String())
}

func (h *MusicHandler) HandleNowPlaying(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.checkAuth(s, i) {
		return
	}

	track, paused, position := h.music.GetNowPlaying(i.GuildID)
	if track == nil {
		respond(s, i, "❌ Tidak ada lagu yang sedang diputar.")
		return
	}

	status := "▶️ Playing"
	if paused {
		status = "⏸️ Paused"
	}
	
	msg := fmt.Sprintf("%s\n🎵 **%s** — %s\n⏳ Posisi: %s", 
		status, track.Info.Title, track.Info.Author, position.String())
	respond(s, i, msg)
}

func formatTrackInfo(track lavalink.Track) string {
	title := track.Info.Title
	if track.Info.URI != nil {
		title = fmt.Sprintf("[%s](<%s>)", title, *track.Info.URI)
	}
	return fmt.Sprintf("**%s** — %s", title, track.Info.Author)
}
