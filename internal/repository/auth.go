package repository

import (
	"context"
	"database/sql"
	"log"
)

type AuthRepository struct {
	DB *sql.DB
}

func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{
		DB: db,
	}
}

func (ar *AuthRepository) IsAuthorized(ctx context.Context, userID string) bool {
	var count int
	err := ar.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorized_users WHERE discord_id = $1", userID).Scan(&count)
	if err != nil {
		log.Println("Error checking authorization in DB:", err)
		return false
	}
	return count > 0
}

func (ar *AuthRepository) GrantAccess(ctx context.Context, targetUserID, grantedBy string) error {
	_, err := ar.DB.ExecContext(ctx, 
		"INSERT INTO authorized_users (discord_id, granted_by, granted_at) VALUES ($1, $2, NOW()) ON CONFLICT (discord_id) DO NOTHING", 
		targetUserID, grantedBy)
	return err
}

func (ar *AuthRepository) RevokeAccess(ctx context.Context, targetUserID string) error {
	_, err := ar.DB.ExecContext(ctx, "DELETE FROM authorized_users WHERE discord_id = $1", targetUserID)
	return err
}
