package main

import (
	"context"
	"database/sql"
	"log"
)

type AuthManager struct {
	OwnerID string
	DB      *sql.DB
}

func NewAuthManager(ownerID string, db *sql.DB) *AuthManager {
	return &AuthManager{
		OwnerID: ownerID,
		DB:      db,
	}
}

func (am *AuthManager) IsOwner(userID string) bool {
	return am.OwnerID != "" && am.OwnerID == userID
}

func (am *AuthManager) IsAuthorized(ctx context.Context, userID string) bool {
	if am.IsOwner(userID) {
		return true
	}

	var count int
	err := am.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM authorized_users WHERE discord_id = $1", userID).Scan(&count)
	if err != nil {
		log.Println("Error checking authorization in DB:", err)
		return false
	}

	return count > 0
}

func (am *AuthManager) GrantAccess(ctx context.Context, targetUserID, grantedBy string) error {
	_, err := am.DB.ExecContext(ctx, 
		"INSERT INTO authorized_users (discord_id, granted_by, granted_at) VALUES ($1, $2, NOW()) ON CONFLICT (discord_id) DO NOTHING", 
		targetUserID, grantedBy)
	return err
}

func (am *AuthManager) RevokeAccess(ctx context.Context, targetUserID string) error {
	_, err := am.DB.ExecContext(ctx, "DELETE FROM authorized_users WHERE discord_id = $1", targetUserID)
	return err
}
