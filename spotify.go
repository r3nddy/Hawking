package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

type SpotifyManager struct {
	client *spotify.Client
}

func NewSpotifyManager() *SpotifyManager {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("SPOTIFY_ID")
	}

	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("SPOTIFY_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		log.Println("Spotify credentials not fully set, API client disabled (will use scraper fallback)")
		return &SpotifyManager{client: nil}
	}

	ctx := context.Background()
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}

	token, err := config.Token(ctx)
	if err != nil {
		log.Println("Failed to obtain Spotify token, API client disabled (will use scraper fallback):", err)
		return &SpotifyManager{client: nil}
	}

	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	log.Println("Spotify API client successfully initialized")
	return &SpotifyManager{client: client}
}

func (sm *SpotifyManager) GetTrackInfo(ctx context.Context, trackID string) (string, string, error) {
	if sm.client != nil {
		id := spotify.ID(trackID)
		track, err := sm.client.GetTrack(ctx, id)
		if err == nil {
			artistName := ""
			if len(track.Artists) > 0 {
				artistName = track.Artists[0].Name
			}
			return track.Name, artistName, nil
		}
		log.Println("Spotify API error (possibly Premium restrictions), falling back to scraping:", err)
	}

	return sm.GetTrackInfoScrape(ctx, trackID)
}

func (sm *SpotifyManager) GetTrackInfoScrape(ctx context.Context, trackID string) (string, string, error) {
	url := "https://open.spotify.com/track/" + trackID
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("spotify web returned status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", "", err
	}
	bodyStr := string(bodyBytes)

	titleReg := regexp.MustCompile(`<meta\s+property="og:title"\s+content="([^"]+)"`)
	titleMatches := titleReg.FindStringSubmatch(bodyStr)
	if len(titleMatches) < 2 {
		titleReg = regexp.MustCompile(`<meta\s+name="twitter:title"\s+content="([^"]+)"`)
		titleMatches = titleReg.FindStringSubmatch(bodyStr)
	}

	descReg := regexp.MustCompile(`<meta\s+property="og:description"\s+content="([^"]+)"`)
	descMatches := descReg.FindStringSubmatch(bodyStr)
	if len(descMatches) < 2 {
		descReg = regexp.MustCompile(`<meta\s+name="twitter:description"\s+content="([^"]+)"`)
		descMatches = descReg.FindStringSubmatch(bodyStr)
	}

	if len(titleMatches) < 2 {
		return "", "", fmt.Errorf("could not extract song title from page")
	}

	title := html.UnescapeString(titleMatches[1])
	artist := "Unknown Artist"

	if len(descMatches) >= 2 {
		desc := html.UnescapeString(descMatches[1])
		
		parts := strings.Split(desc, " · ")
		if len(parts) >= 1 {
			artist = parts[0]
			
			if strings.Contains(artist, "on Spotify.") {
				subParts := strings.Split(artist, "on Spotify. ")
				if len(subParts) > 1 {
					artist = subParts[1]
				}
			}
		}
	}

	return title, artist, nil
}

func ExtractTrackID(url string) string {
	startIdx := -1
	for i := 0; i < len(url)-6; i++ {
		if url[i:i+6] == "track/" {
			startIdx = i + 6
			break
		}
	}
	if startIdx == -1 {
		return ""
	}

	endIdx := len(url)
	for i := startIdx; i < len(url); i++ {
		if url[i] == '?' {
			endIdx = i
			break
		}
	}

	return url[startIdx:endIdx]
}
