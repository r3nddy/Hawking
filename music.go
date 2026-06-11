package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgolink/v3/disgolink"
	"github.com/disgoorg/disgolink/v3/lavalink"
	"github.com/disgoorg/snowflake/v2"
)

type PlayResult struct {
	Action string
	Track  lavalink.Track
}

type MusicManager struct {
	client  disgolink.Client
	spotify *SpotifyManager
	queues  map[string][]lavalink.Track
	mu      sync.Mutex
}

var defaultNodes = []disgolink.NodeConfig{
	{Name: "jirayu", Address: "lavalink.jirayu.net:443", Password: "youshallnotpass", Secure: true},
	{Name: "serenetia", Address: "lavalinkv4.serenetia.com:443", Password: "https://seretia.link/discord", Secure: true},
	{Name: "millohost", Address: "lava-v4.millohost.my.id:443", Password: "https://discord.gg/mjS5J2K3ep", Secure: true},
}

func NewMusicManager(botUserID string, spotify *SpotifyManager) (*MusicManager, error) {
	botID, err := snowflake.Parse(botUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid bot user ID: %w", err)
	}

	m := &MusicManager{
		spotify: spotify,
		queues:  make(map[string][]lavalink.Track),
	}

	m.client = disgolink.New(botID,
		disgolink.WithListenerFunc(m.onTrackEnd),
		disgolink.WithListenerFunc(m.onTrackException),
		disgolink.WithListenerFunc(m.onWebSocketClosed),
	)
	return m, nil
}

func (m *MusicManager) Client() disgolink.Client {
	return m.client
}

func (m *MusicManager) Connect(ctx context.Context) error {
	if host := os.Getenv("LAVALINK_HOST"); host != "" {
		port := os.Getenv("LAVALINK_PORT")
		if port == "" {
			port = "443"
		}
		password := os.Getenv("LAVALINK_PASSWORD")
		if password == "" {
			password = "youshallnotpass"
		}
		secure := os.Getenv("LAVALINK_SECURE") != "false"

		_, err := m.client.AddNode(ctx, disgolink.NodeConfig{
			Name:     "default",
			Address:  host + ":" + port,
			Password: password,
			Secure:   secure,
		})
		return err
	}

	var lastErr error
	for _, node := range defaultNodes {
		_, err := m.client.AddNode(ctx, node)
		if err == nil {
			log.Printf("Connected to Lavalink node: %s", node.Name)
			return nil
		}
		lastErr = err
		log.Printf("Failed to connect to Lavalink node %s: %v", node.Name, err)
	}
	return fmt.Errorf("failed to connect to any Lavalink node: %w", lastErr)
}

func (m *MusicManager) Play(ctx context.Context, guildID, query string) (*PlayResult, error) {
	searchQuery, err := m.resolveQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	node := m.client.BestNode()
	if node == nil {
		return nil, fmt.Errorf("no Lavalink node available")
	}

	var track lavalink.Track
	var loaded bool
	var loadErr error

	node.LoadTracksHandler(ctx, searchQuery, disgolink.NewResultHandler(
		func(t lavalink.Track) {
			track = t
			loaded = true
		},
		func(playlist lavalink.Playlist) {
			if len(playlist.Tracks) > 0 {
				track = playlist.Tracks[0]
				loaded = true
			}
		},
		func(tracks []lavalink.Track) {
			if len(tracks) > 0 {
				track = tracks[0]
				loaded = true
			}
		},
		func() {
			loadErr = fmt.Errorf("no tracks found for: %s", query)
		},
		func(err error) {
			loadErr = err
		},
	))

	if loadErr != nil {
		return nil, loadErr
	}
	if !loaded {
		return nil, fmt.Errorf("no tracks found for: %s", query)
	}

	sfGuildID, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, err
	}

	player := m.client.Player(sfGuildID)
	if player.Track() != nil {
		m.addToQueue(guildID, track)
		return &PlayResult{Action: "queued", Track: track}, nil
	}

	if err := m.waitForVoice(ctx, sfGuildID); err != nil {
		return nil, err
	}

	if err := m.playTrack(ctx, sfGuildID, track); err != nil {
		return nil, err
	}
	return &PlayResult{Action: "playing", Track: track}, nil
}

func (m *MusicManager) Pause(ctx context.Context, guildID string) error {
	player, err := m.getPlayer(guildID)
	if err != nil {
		return err
	}
	if player.Track() == nil {
		return fmt.Errorf("nothing is playing")
	}
	return player.Update(ctx, lavalink.WithPaused(true))
}

func (m *MusicManager) Resume(ctx context.Context, guildID string) error {
	player, err := m.getPlayer(guildID)
	if err != nil {
		return err
	}
	if player.Track() == nil {
		return fmt.Errorf("nothing is playing")
	}
	return player.Update(ctx, lavalink.WithPaused(false))
}

func (m *MusicManager) Skip(ctx context.Context, guildID string) (*lavalink.Track, error) {
	player, err := m.getPlayer(guildID)
	if err != nil {
		return nil, err
	}
	if player.Track() == nil {
		return nil, fmt.Errorf("nothing is playing")
	}

	next := m.nextTrack(guildID)
	if next == nil {
		if err := player.Update(ctx, lavalink.WithNullTrack()); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if err := player.Update(ctx, lavalink.WithTrack(*next)); err != nil {
		return nil, err
	}
	return next, nil
}

func (m *MusicManager) Stop(ctx context.Context, guildID string) error {
	m.clearQueue(guildID)

	sfGuildID, err := snowflake.Parse(guildID)
	if err != nil {
		return err
	}

	player := m.client.ExistingPlayer(sfGuildID)
	if player == nil || player.Track() == nil {
		return nil
	}
	return player.Update(ctx, lavalink.WithNullTrack())
}

func (m *MusicManager) GetQueue(guildID string) []lavalink.Track {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]lavalink.Track(nil), m.queues[guildID]...)
}

func (m *MusicManager) GetNowPlaying(guildID string) (*lavalink.Track, bool, lavalink.Duration) {
	sfGuildID, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, false, 0
	}

	player := m.client.ExistingPlayer(sfGuildID)
	if player == nil || player.Track() == nil {
		return nil, false, 0
	}

	return player.Track(), player.Paused(), player.Position()
}

func (m *MusicManager) ClearGuild(guildID string) {
	m.clearQueue(guildID)
}

func (m *MusicManager) onTrackEnd(player disgolink.Player, event lavalink.TrackEndEvent) {
	log.Printf("Track ended in guild %s: %s — %s (reason: %s)",
		player.GuildID(), event.Track.Info.Title, event.Track.Info.Author, event.Reason)

	if !event.Reason.MayStartNext() {
		return
	}

	guildID := player.GuildID().String()
	next := m.nextTrack(guildID)
	if next == nil {
		return
	}

	if err := m.playTrack(context.Background(), player.GuildID(), *next); err != nil {
		log.Printf("Failed to play next track in guild %s: %v", guildID, err)
	}
}

func (m *MusicManager) resolveQuery(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	if strings.Contains(query, "open.spotify.com/track/") {
		if m.spotify == nil {
			return "", fmt.Errorf("Spotify API belum dikonfigurasi")
		}
		trackID := ExtractTrackID(query)
		if trackID == "" {
			return "", fmt.Errorf("URL Spotify tidak valid")
		}
		title, artist, err := m.spotify.GetTrackInfo(ctx, trackID)
		if err != nil {
			return "", fmt.Errorf("gagal mendapatkan info lagu Spotify: %w", err)
		}
		return "ytsearch:" + title + " " + artist, nil
	}

	lower := strings.ToLower(query)
	if strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be") ||
		strings.Contains(lower, "soundcloud.com") {
		return query, nil
	}

	if strings.HasPrefix(lower, "ytsearch:") || strings.HasPrefix(lower, "scsearch:") {
		return query, nil
	}

	return "ytsearch:" + query, nil
}

func (m *MusicManager) onTrackException(player disgolink.Player, event lavalink.TrackExceptionEvent) {
	log.Printf("Track exception in guild %s: %s — %v",
		player.GuildID(), event.Track.Info.Title, event.Exception)
}

func (m *MusicManager) onWebSocketClosed(player disgolink.Player, event lavalink.WebSocketClosedEvent) {
	log.Printf("Voice websocket closed in guild %s: code=%d reason=%s remote=%v",
		player.GuildID(), event.Code, event.Reason, event.ByRemote)
}

func (m *MusicManager) waitForVoice(ctx context.Context, guildID snowflake.ID) error {
	deadline := time.Now().Add(15 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		player := m.client.ExistingPlayer(guildID)
		if player != nil && player.ChannelID() != nil && player.State().Connected {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout menunggu koneksi voice Lavalink (pastikan bot sudah di voice channel)")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *MusicManager) playTrack(ctx context.Context, guildID snowflake.ID, track lavalink.Track) error {
	player := m.client.Player(guildID)
	log.Printf("Playing track in guild %s: %s — %s", guildID, track.Info.Title, track.Info.Author)
	return player.Update(ctx, lavalink.WithTrack(track))
}

func (m *MusicManager) getPlayer(guildID string) (disgolink.Player, error) {
	sfGuildID, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, err
	}

	player := m.client.ExistingPlayer(sfGuildID)
	if player == nil {
		return nil, fmt.Errorf("bot belum terhubung ke voice channel")
	}
	return player, nil
}

func (m *MusicManager) addToQueue(guildID string, track lavalink.Track) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queues[guildID] = append(m.queues[guildID], track)
}

func (m *MusicManager) nextTrack(guildID string) *lavalink.Track {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := m.queues[guildID]
	if len(queue) == 0 {
		return nil
	}

	track := queue[0]
	m.queues[guildID] = queue[1:]
	return &track
}

func (m *MusicManager) clearQueue(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.queues, guildID)
}

func formatTrack(track lavalink.Track) string {
	if track.Info.Author != "" {
		return fmt.Sprintf("**%s** — %s", track.Info.Title, track.Info.Author)
	}
	return fmt.Sprintf("**%s**", track.Info.Title)
}

func formatDuration(d lavalink.Duration) string {
	totalSec := d.Seconds()
	min := totalSec / 60
	sec := totalSec % 60
	return fmt.Sprintf("%d:%02d", min, sec)
}
