package handlers

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"hawking-bot/internal/discord"
	"hawking-bot/internal/services"
)

type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService, router *discord.Router) *AuthHandler {
	h := &AuthHandler{svc: svc}

	router.Register(&discordgo.ApplicationCommand{
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
	}, h.HandleGrant)

	router.Register(&discordgo.ApplicationCommand{
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
	}, h.HandleRevoke)

	return h
}

func (h *AuthHandler) HandleGrant(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.svc.IsOwner(i.Member.User.ID) {
		respondEphemeral(s, i, "❌ Hanya Owner bot yang dapat menggunakan command ini.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	err := h.svc.GrantAccess(context.Background(), targetUser.ID, i.Member.User.ID)
	if err != nil {
		respondEphemeral(s, i, "❌ Gagal memberikan izin ke database.")
		return
	}

	respond(s, i, fmt.Sprintf("✅ Berhasil memberikan izin akses kepada <@%s>", targetUser.ID))
}

func (h *AuthHandler) HandleRevoke(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.svc.IsOwner(i.Member.User.ID) {
		respondEphemeral(s, i, "❌ Hanya Owner bot yang dapat menggunakan command ini.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	err := h.svc.RevokeAccess(context.Background(), targetUser.ID)
	if err != nil {
		respondEphemeral(s, i, "❌ Gagal mencabut izin dari database.")
		return
	}

	respond(s, i, fmt.Sprintf("✅ Berhasil mencabut izin akses dari <@%s>", targetUser.ID))
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}
