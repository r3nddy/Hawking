package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

type SpotifyManager struct {
	client *spotify.Client
}

func NewSpotifyManager() (*SpotifyManager, error) {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("SPOTIFY_ID")
	}

	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("SPOTIFY_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		return nil, errors.New("neither SPOTIFY_CLIENT_ID nor SPOTIFY_ID, and/or SPOTIFY_CLIENT_SECRET nor SPOTIFY_SECRET are set in env")
	}

	ctx := context.Background()
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}

	token, err := config.Token(ctx)
	if err != nil {
		return nil, err
	}

	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	log.Println("Spotify client successfully initialized")
	return &SpotifyManager{client: client}, nil
}

func (sm *SpotifyManager) GetTrackInfo(ctx context.Context, trackID string) (string, string, error) {
	id := spotify.ID(trackID)
	track, err := sm.client.GetTrack(ctx, id)
	if err != nil {
		return "", "", err
	}

	artistName := ""
	if len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}

	return track.Name, artistName, nil
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
