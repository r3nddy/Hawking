package services

import (
	"context"
	"hawking-bot/internal/repository"
)

type AuthService struct {
	repo    *repository.AuthRepository
	ownerID string
}

func NewAuthService(repo *repository.AuthRepository, ownerID string) *AuthService {
	return &AuthService{
		repo:    repo,
		ownerID: ownerID,
	}
}

func (s *AuthService) IsOwner(userID string) bool {
	return s.ownerID != "" && s.ownerID == userID
}

func (s *AuthService) IsAuthorized(ctx context.Context, userID string) bool {
	if s.IsOwner(userID) {
		return true
	}
	return s.repo.IsAuthorized(ctx, userID)
}

func (s *AuthService) GrantAccess(ctx context.Context, targetUserID, grantedBy string) error {
	return s.repo.GrantAccess(ctx, targetUserID, grantedBy)
}

func (s *AuthService) RevokeAccess(ctx context.Context, targetUserID string) error {
	return s.repo.RevokeAccess(ctx, targetUserID)
}
