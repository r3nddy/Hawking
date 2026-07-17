package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgolink/v3/disgolink"
	"github.com/disgoorg/disgolink/v3/lavalink"
	"github.com/disgoorg/snowflake/v2"
	"hawking-bot/internal/models"
)

type PlayResult struct {
	Action string
	Track  lavalink.Track
}

type CycleStatus struct {
	Enabled bool
	Count   int
}

type voiceGate struct {
	mu     sync.Mutex
	state  bool
	server bool
	once   sync.Once
	ready  chan struct{}
}

func newVoiceGate() *voiceGate {
	return &voiceGate{ready: make(chan struct{})}
}

func (g *voiceGate) markState()  { g.mark(true, false) }
func (g *voiceGate) markServer() { g.mark(false, true) }

func (g *voiceGate) mark(state, server bool) {
	g.mu.Lock()
	if state {
		g.state = true
	}
	if server {
		g.server = true
	}
	ready := g.state && g.server
	g.mu.Unlock()
	if ready {
		g.once.Do(func() { close(g.ready) })
	}
}

type MusicService struct {
	client      disgolink.Client
	spotify     *SpotifyService
	queues      map[string][]lavalink.Track
	cycles      map[string][]lavalink.Track
	cycleIndex  map[string]int
	mu          sync.Mutex
	playMu      sync.Mutex
	playWaiters map[string]chan error
	voiceMu     sync.Mutex
	voiceGates  map[string]*voiceGate
}

var defaultNodes = []disgolink.NodeConfig{
	{Name: "serenetia", Address: "lavalinkv4.serenetia.com:443", Password: "https://seretia.link/discord", Secure: true},
	{Name: "millohost", Address: "lava-v4.millohost.my.id:443", Password: "https://discord.gg/mjS5J2K3ep", Secure: true},
	{Name: "jirayu", Address: "lavalink.jirayu.net:443", Password: "youshallnotpass", Secure: true},
}

func NewMusicService(botUserID string, spotify *SpotifyService) (*MusicService, error) {
	botID, err := snowflake.Parse(botUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid bot user ID: %w", err)
	}

	m := &MusicService{
		spotify:    spotify,
		queues:     make(map[string][]lavalink.Track),
		cycles:     make(map[string][]lavalink.Track),
		cycleIndex: make(map[string]int),
	}

	m.client = disgolink.New(botID,
		disgolink.WithListenerFunc(m.onTrackStart),
		disgolink.WithListenerFunc(m.onTrackEnd),
		disgolink.WithListenerFunc(m.onTrackException),
		disgolink.WithListenerFunc(m.onWebSocketClosed),
		disgolink.WithListenerFunc(m.onPlayerUpdate),
	)
	return m, nil
}

func (m *MusicService) Client() disgolink.Client {
	return m.client
}

func (m *MusicService) Connect(ctx context.Context) error {
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

		return m.connectNode(ctx, disgolink.NodeConfig{
			Name:     "default",
			Address:  host + ":" + port,
			Password: password,
			Secure:   secure,
		})
	}

	var lastErr error
	for _, node := range defaultNodes {
		err := m.connectNode(ctx, node)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Skipping Lavalink node %s: %v", node.Name, err)
	}
	return fmt.Errorf("tidak ada node Lavalink yang kompatibel (butuh v4.2.0+ untuk DAVE): %w", lastErr)
}

func (m *MusicService) connectNode(ctx context.Context, cfg disgolink.NodeConfig) error {
	node, err := m.client.AddNode(ctx, cfg)
	if err != nil {
		return err
	}

	version, source, err := getNodeVersion(ctx, node)
	if err != nil {
		m.client.RemoveNode(cfg.Name)
		return fmt.Errorf("tidak bisa verifikasi versi: %w", err)
	}

	log.Printf("Lavalink node %s: v%s (via %s)", cfg.Name, version, source)

	if !lavalinkVersionSupportsDAVE(ctx, node) {
		m.client.RemoveNode(cfg.Name)
		return fmt.Errorf("v%s tidak support DAVE (butuh Lavalink 4.2.0+)", version)
	}

	log.Printf("Using Lavalink node %s (v%s)", cfg.Name, version)
	return nil
}

func getNodeVersion(ctx context.Context, node disgolink.Node) (string, string, error) {
	if version, err := node.Version(ctx); err == nil {
		version = strings.TrimSpace(version)
		if version != "" {
			return version, "version", nil
		}
	}

	info, err := node.Info(ctx)
	if err != nil {
		return "", "", err
	}
	if info.Version.Semver != "" {
		return info.Version.Semver, "info", nil
	}
	return fmt.Sprintf("%d.%d.%d", info.Version.Major, info.Version.Minor, info.Version.Patch), "info", nil
}

func lavalinkVersionSupportsDAVE(ctx context.Context, node disgolink.Node) bool {
	// Coba pakai node.Info() karena punya Major/Minor structured integer
	// (tidak tergantung string parsing yang bisa gagal buat SNAPSHOT builds)
	info, err := node.Info(ctx)
	if err == nil {
		major := info.Version.Major
		minor := info.Version.Minor
		return major > 4 || (major == 4 && minor >= 2)
	}

	// Fallback: parse string version (untuk node yang tidak support Info endpoint)
	version, err := node.Version(ctx)
	if err != nil {
		return false
	}

	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	return major > 4 || (major == 4 && minor >= 2)
}

func (m *MusicService) Play(ctx context.Context, guildID, query string) (*PlayResult, error) {
	track, err := m.ResolveTrack(ctx, query)
	if err != nil {
		return nil, err
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

	if err := m.waitForVoice(ctx, guildID, sfGuildID); err != nil {
		return nil, err
	}

	waitPlayback, cleanup := m.registerPlayWait(guildID)
	defer cleanup()

	if err := m.playTrack(ctx, sfGuildID, track); err != nil {
		return nil, err
	}
	if err := waitPlayback(ctx); err != nil {
		return nil, err
	}
	return &PlayResult{Action: "playing", Track: track}, nil
}

func (m *MusicService) StartCycle(ctx context.Context, guildID string, savedTracks []models.Track) (*PlayResult, int, error) {
	if len(savedTracks) == 0 {
		return nil, 0, fmt.Errorf("playlist tersimpan kosong")
	}

	tracks := make([]lavalink.Track, 0, len(savedTracks))
	for _, saved := range savedTracks {
		query := strings.TrimSpace(saved.TrackTitle + " " + saved.TrackArtist)
		if strings.TrimSpace(query) == "" {
			query = saved.SpotifyURL
		}

		track, err := m.ResolveTrack(ctx, query)
		if err != nil {
			return nil, len(tracks), fmt.Errorf("gagal resolve %s - %s: %w", saved.TrackTitle, saved.TrackArtist, err)
		}
		tracks = append(tracks, track)
	}

	sfGuildID, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, len(tracks), err
	}

	if err := m.waitForVoice(ctx, guildID, sfGuildID); err != nil {
		return nil, len(tracks), err
	}

	m.startCycle(guildID, tracks)

	waitPlayback, cleanup := m.registerPlayWait(guildID)
	defer cleanup()

	if err := m.playTrack(ctx, sfGuildID, tracks[0]); err != nil {
		m.StopCycle(guildID)
		return nil, len(tracks), err
	}
	if err := waitPlayback(ctx); err != nil {
		m.StopCycle(guildID)
		return nil, len(tracks), err
	}

	return &PlayResult{Action: "cycle", Track: tracks[0]}, len(tracks), nil
}

func (m *MusicService) Pause(ctx context.Context, guildID string) error {
	player, err := m.getPlayer(guildID)
	if err != nil {
		return err
	}
	if player.Track() == nil {
		return fmt.Errorf("nothing is playing")
	}
	return player.Update(ctx, lavalink.WithPaused(true))
}

func (m *MusicService) Resume(ctx context.Context, guildID string) error {
	player, err := m.getPlayer(guildID)
	if err != nil {
		return err
	}
	if player.Track() == nil {
		return fmt.Errorf("nothing is playing")
	}
	return player.Update(ctx, lavalink.WithPaused(false))
}

func (m *MusicService) Skip(ctx context.Context, guildID string) (*lavalink.Track, error) {
	player, err := m.getPlayer(guildID)
	if err != nil {
		return nil, err
	}
	if player.Track() == nil {
		return nil, fmt.Errorf("nothing is playing")
	}

	next := m.nextPlayableTrack(guildID)
	if next == nil {
		if err := player.Update(ctx, lavalink.WithNullTrack()); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if err := m.playTrack(ctx, player.GuildID(), *next); err != nil {
		return nil, err
	}
	return next, nil
}

func (m *MusicService) Stop(ctx context.Context, guildID string) error {
	m.clearQueue(guildID)
	m.StopCycle(guildID)

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

func (m *MusicService) GetQueue(guildID string) []lavalink.Track {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]lavalink.Track(nil), m.queues[guildID]...)
}

func (m *MusicService) GetCycleStatus(guildID string) CycleStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	cycle := m.cycles[guildID]
	return CycleStatus{
		Enabled: len(cycle) > 0,
		Count:   len(cycle),
	}
}

func (m *MusicService) GetNowPlaying(guildID string) (*lavalink.Track, bool, lavalink.Duration) {
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

func (m *MusicService) ClearGuild(guildID string) {
	m.clearQueue(guildID)
	m.StopCycle(guildID)
}

func (m *MusicService) ResetVoiceGate(guildID string) {
	m.voiceMu.Lock()
	defer m.voiceMu.Unlock()
	if m.voiceGates == nil {
		m.voiceGates = make(map[string]*voiceGate)
	}
	m.voiceGates[guildID] = newVoiceGate()
}

func (m *MusicService) NotifyVoiceState(guildID string) {
	m.getVoiceGate(guildID).markState()
}

func (m *MusicService) NotifyVoiceServer(guildID string) {
	m.getVoiceGate(guildID).markServer()
}

func (m *MusicService) getVoiceGate(guildID string) *voiceGate {
	m.voiceMu.Lock()
	defer m.voiceMu.Unlock()
	if m.voiceGates == nil {
		m.voiceGates = make(map[string]*voiceGate)
	}
	if g, ok := m.voiceGates[guildID]; ok {
		return g
	}
	g := newVoiceGate()
	m.voiceGates[guildID] = g
	return g
}

func (m *MusicService) onPlayerUpdate(player disgolink.Player, event lavalink.PlayerUpdateMessage) {
	log.Printf("[voice] PlayerUpdate guild=%s connected=%v ping=%dms",
		player.GuildID(), event.State.Connected, event.State.Ping)
}

func (m *MusicService) onTrackStart(player disgolink.Player, event lavalink.TrackStartEvent) {
	state := player.State()
	log.Printf("Track started in guild %s: %s — %s (connected=%v ping=%dms)",
		player.GuildID(), event.Track.Info.Title, event.Track.Info.Author, state.Connected, state.Ping)
	m.signalPlayWait(player.GuildID().String(), nil)
}

func (m *MusicService) onTrackEnd(player disgolink.Player, event lavalink.TrackEndEvent) {
	log.Printf("Track ended in guild %s: %s — %s (reason: %s)",
		player.GuildID(), event.Track.Info.Title, event.Track.Info.Author, event.Reason)

	if event.Reason == lavalink.TrackEndReasonLoadFailed {
		m.signalPlayWait(player.GuildID().String(), fmt.Errorf("gagal memutar audio (%s)", event.Reason))
	}

	if !event.Reason.MayStartNext() {
		return
	}

	guildID := player.GuildID().String()
	next := m.nextPlayableTrack(guildID)
	if next == nil {
		return
	}

	if err := m.playTrack(context.Background(), player.GuildID(), *next); err != nil {
		log.Printf("Failed to play next track in guild %s: %v", guildID, err)
	}
}

func (m *MusicService) ResolveTrack(ctx context.Context, query string) (lavalink.Track, error) {
	searchQuery, err := m.resolveQuery(ctx, query)
	if err != nil {
		return lavalink.Track{}, err
	}

	node := m.client.BestNode()
	if node == nil {
		return lavalink.Track{}, fmt.Errorf("no Lavalink node available")
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
		return lavalink.Track{}, loadErr
	}
	if !loaded {
		return lavalink.Track{}, fmt.Errorf("no tracks found for: %s", query)
	}

	return track, nil
}

func (m *MusicService) resolveQuery(ctx context.Context, query string) (string, error) {
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

func (m *MusicService) onTrackException(player disgolink.Player, event lavalink.TrackExceptionEvent) {
	log.Printf("Track exception in guild %s: %s — %v",
		player.GuildID(), event.Track.Info.Title, event.Exception)
	m.signalPlayWait(player.GuildID().String(), fmt.Errorf("track exception: %s", event.Exception.Message))
}

func (m *MusicService) onWebSocketClosed(player disgolink.Player, event lavalink.WebSocketClosedEvent) {
	log.Printf("Voice websocket closed in guild %s: code=%d reason=%s remote=%v",
		player.GuildID(), event.Code, event.Reason, event.ByRemote)

	msg := fmt.Sprintf("koneksi voice terputus: %s (code %d)", event.Reason, event.Code)
	if event.Code == 4017 {
		msg = "Discord menolak koneksi voice (DAVE/E2EE required). Node Lavalink harus v4.2.0+"
	}
	m.signalPlayWait(player.GuildID().String(), fmt.Errorf("%s", msg))
}

func (m *MusicService) registerPlayWait(guildID string) (func(context.Context) error, func()) {
	ch := make(chan error, 1)
	m.playMu.Lock()
	if m.playWaiters == nil {
		m.playWaiters = make(map[string]chan error)
	}
	m.playWaiters[guildID] = ch
	m.playMu.Unlock()

	cleanup := func() {
		m.playMu.Lock()
		delete(m.playWaiters, guildID)
		m.playMu.Unlock()
	}

	wait := func(ctx context.Context) error {
		select {
		case err := <-ch:
			return err
		case <-time.After(15 * time.Second):
			return fmt.Errorf("audio tidak mulai — pastikan node Lavalink v4.2.0+ (DAVE) dan bot sudah di voice channel")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return wait, cleanup
}

func (m *MusicService) signalPlayWait(guildID string, err error) {
	m.playMu.Lock()
	ch, ok := m.playWaiters[guildID]
	m.playMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- err:
	default:
	}
}

func (m *MusicService) waitForVoice(ctx context.Context, guildID string, sfGuildID snowflake.ID) error {
	if player := m.client.ExistingPlayer(sfGuildID); player != nil && player.ChannelID() != nil {
		if player.State().Connected {
			return nil
		}
	}

	gate := m.getVoiceGate(guildID)
	deadline := time.After(20 * time.Second)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	voiceEventsReady := false
	ready := gate.ready

	for {
		select {
		case <-ready:
			voiceEventsReady = true
			ready = nil
			log.Printf("[voice] guild %s: Discord voice events received", guildID)
		case <-deadline:
			player := m.client.ExistingPlayer(sfGuildID)
			if player != nil && player.ChannelID() != nil && player.State().Connected {
				return nil
			}
			if voiceEventsReady {
				return fmt.Errorf("timeout menunggu Lavalink siap mengirim audio (voice events diterima, connected=false)")
			}
			return fmt.Errorf("timeout menunggu koneksi voice Lavalink (pastikan bot sudah di voice channel)")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			player := m.client.ExistingPlayer(sfGuildID)
			if player == nil {
				continue
			}
			if player.State().Connected {
				log.Printf("[voice] guild %s: Lavalink voice connected", guildID)
				return nil
			}
		}
	}
}

func (m *MusicService) playTrack(ctx context.Context, guildID snowflake.ID, track lavalink.Track) error {
	player := m.client.Player(guildID)
	log.Printf("Playing track in guild %s: %s — %s", guildID, track.Info.Title, track.Info.Author)
	return player.Update(ctx, lavalink.WithTrack(track), lavalink.WithVolume(100), lavalink.WithPaused(false))
}

func (m *MusicService) getPlayer(guildID string) (disgolink.Player, error) {
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

func (m *MusicService) addToQueue(guildID string, track lavalink.Track) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queues[guildID] = append(m.queues[guildID], track)
}

func (m *MusicService) nextTrack(guildID string) *lavalink.Track {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.nextTrackLocked(guildID)
}

func (m *MusicService) nextPlayableTrack(guildID string) *lavalink.Track {
	m.mu.Lock()
	defer m.mu.Unlock()

	if track := m.nextTrackLocked(guildID); track != nil {
		return track
	}
	return m.nextCycleTrackLocked(guildID)
}

func (m *MusicService) nextTrackLocked(guildID string) *lavalink.Track {
	queue := m.queues[guildID]
	if len(queue) == 0 {
		return nil
	}

	track := queue[0]
	m.queues[guildID] = queue[1:]
	return &track
}

func (m *MusicService) startCycle(guildID string, tracks []lavalink.Track) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queues[guildID] = nil
	m.cycles[guildID] = append([]lavalink.Track(nil), tracks...)
	m.cycleIndex[guildID] = 1
}

func (m *MusicService) StopCycle(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cycles, guildID)
	delete(m.cycleIndex, guildID)
}

func (m *MusicService) nextCycleTrackLocked(guildID string) *lavalink.Track {
	cycle := m.cycles[guildID]
	if len(cycle) == 0 {
		return nil
	}

	idx := m.cycleIndex[guildID]
	if idx < 0 || idx >= len(cycle) {
		idx = 0
	}

	track := cycle[idx]
	m.cycleIndex[guildID] = (idx + 1) % len(cycle)
	return &track
}

func (m *MusicService) clearQueue(guildID string) {
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

