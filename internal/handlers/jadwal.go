package handlers

import (
	"github.com/bwmarrin/discordgo"
	"hawking-bot/internal/discord"
	"hawking-bot/internal/services"
)

type JadwalHandler struct {
	svc *services.JadwalService
}

func NewJadwalHandler(svc *services.JadwalService, router *discord.Router) *JadwalHandler {
	h := &JadwalHandler{svc: svc}
	
	router.Register(&discordgo.ApplicationCommand{
		Name:        "jadwal",
		Description: "Tampilkan jadwal kuliah kelas B 25",
	}, h.HandleJadwal)

	return h
}

func (h *JadwalHandler) HandleJadwal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: h.svc.GetFormattedJadwal(),
		},
	})
}
