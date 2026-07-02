package repository

import (
	"context"
	"database/sql"
	"log"

	"hawking-bot/internal/models"
)

type StorageRepository struct {
	db *sql.DB
}

func NewStorageRepository(db *sql.DB) *StorageRepository {
	return &StorageRepository{db: db}
}

func (sm *StorageRepository) SaveTrack(ctx context.Context, discordID, title, artist, url string) error {
	_, err := sm.db.ExecContext(ctx,
		"INSERT INTO saved_tracks (discord_id, track_title, track_artist, spotify_url) VALUES ($1, $2, $3, $4)",
		discordID, title, artist, url)
	return err
}

func (sm *StorageRepository) DeleteTrack(ctx context.Context, discordID string, id int) (bool, error) {
	result, err := sm.db.ExecContext(ctx,
		"DELETE FROM saved_tracks WHERE id = $1 AND discord_id = $2",
		id, discordID)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (sm *StorageRepository) CountTracks(ctx context.Context, discordID string) (int, error) {
	var total int
	err := sm.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM saved_tracks WHERE discord_id = $1",
		discordID).Scan(&total)
	return total, err
}

func (sm *StorageRepository) GetTracksPage(ctx context.Context, discordID string, limit, offset int) ([]models.Track, error) {
	rows, err := sm.db.QueryContext(ctx,
		"SELECT id, track_title, track_artist, spotify_url FROM saved_tracks WHERE discord_id = $1 ORDER BY added_at DESC LIMIT $2 OFFSET $3",
		discordID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		if err := rows.Scan(&t.ID, &t.TrackTitle, &t.TrackArtist, &t.SpotifyURL); err != nil {
			log.Println("Error scanning track row:", err)
			continue
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tracks, nil
}

func (sm *StorageRepository) GetTracks(ctx context.Context, discordID string) ([]models.Track, error) {
	rows, err := sm.db.QueryContext(ctx,
		"SELECT id, track_title, track_artist, spotify_url FROM saved_tracks WHERE discord_id = $1 ORDER BY added_at DESC",
		discordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		if err := rows.Scan(&t.ID, &t.TrackTitle, &t.TrackArtist, &t.SpotifyURL); err != nil {
			log.Println("Error scanning track row:", err)
			continue
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tracks, nil
}
