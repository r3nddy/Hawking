package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"hawking-bot/internal/discord"
	"hawking-bot/internal/models"
	"hawking-bot/internal/services"
)

type ReminderHandler struct {
	reminderSvc *services.ReminderService
	authSvc     *services.AuthService
}

func NewReminderHandler(
	reminderSvc *services.ReminderService,
	authSvc *services.AuthService,
	router *discord.Router,
) *ReminderHandler {
	h := &ReminderHandler{
		reminderSvc: reminderSvc,
		authSvc:     authSvc,
	}

	// Command: /reminder-setup
	router.Register(&discordgo.ApplicationCommand{
		Name:        "reminder-setup",
		Description: "Setup konfigurasi reminder untuk server ini (Admin only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel untuk mengirim reminder",
				Required:    true,
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildText,
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "waktu",
				Description: "Jam pengiriman reminder (format: HH:MM, contoh: 20:00)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "hari-sebelum",
				Description: "Berapa hari sebelum jadwal (default: 1)",
				Required:    false,
				MinValue:    ptr(0.0),
				MaxValue:    7,
			},
		},
	}, h.HandleReminderSetup)

	// Command: /reminder-toggle
	router.Register(&discordgo.ApplicationCommand{
		Name:        "reminder-toggle",
		Description: "Aktifkan/nonaktifkan reminder (Admin only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "enabled",
				Description: "Aktifkan (true) atau nonaktifkan (false)",
				Required:    true,
			},
		},
	}, h.HandleReminderToggle)

	// Command: /reminder-status
	router.Register(&discordgo.ApplicationCommand{
		Name:        "reminder-status",
		Description: "Lihat status konfigurasi reminder",
	}, h.HandleReminderStatus)

	// Command: /reminder-test
	router.Register(&discordgo.ApplicationCommand{
		Name:        "reminder-test",
		Description: "Test reminder secara manual (Admin only)",
	}, h.HandleReminderTest)

	return h
}

func (h *ReminderHandler) HandleReminderSetup(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	// Cek apakah user adalah admin atau owner
	if !h.isAdmin(ctx, i.Member) {
		respondError(s, i, "❌ Hanya admin yang bisa mengatur konfigurasi reminder!")
		return
	}

	// Defer response karena ini bisa lama
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Parse options
	options := parseOptions(i.ApplicationCommandData().Options)

	channelID := options["channel"].ChannelValue(s).ID

	// Default values
	reminderTime := "20:00:00"
	daysBefore := 1

	// Parse waktu jika ada
	if waktuStr, ok := options["waktu"]; ok {
		parsedTime, err := time.Parse("15:04", waktuStr.StringValue())
		if err != nil {
			editError(s, i, "❌ Format waktu tidak valid! Gunakan format HH:MM (contoh: 20:00)")
			return
		}
		reminderTime = parsedTime.Format("15:04:05")
	}

	// Parse hari-sebelum jika ada
	if hariSebelum, ok := options["hari-sebelum"]; ok {
		daysBefore = int(hariSebelum.IntValue())
	}

	// Parse reminder time ke time.Time
	now := time.Now()
	parsedTime, err := time.Parse("15:04:05", reminderTime)
	if err != nil {
		editError(s, i, "❌ Error parsing waktu!")
		return
	}

	fullTime := time.Date(now.Year(), now.Month(), now.Day(),
		parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, time.Local)

	// Setup config
	config := &models.ReminderConfig{
		GuildID:      i.GuildID,
		ChannelID:    channelID,
		ReminderTime: fullTime,
		IsEnabled:    true,
		DaysBefore:   daysBefore,
	}

	if err := h.reminderSvc.SetupReminderConfig(ctx, config); err != nil {
		editError(s, i, fmt.Sprintf("❌ Gagal menyimpan konfigurasi: %v", err))
		return
	}

	// Success response
	response := fmt.Sprintf(
		"✅ **Konfigurasi Reminder Berhasil!**\n\n"+
			"📍 Channel: <#%s>\n"+
			"⏰ Waktu: %s\n"+
			"📅 Hari sebelum: %d hari\n"+
			"✅ Status: Aktif\n\n"+
			"Reminder akan dikirim otomatis sesuai jadwal yang dikonfigurasi.",
		channelID,
		parsedTime.Format("15:04"),
		daysBefore,
	)

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

func (h *ReminderHandler) HandleReminderToggle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	// Cek apakah user adalah admin atau owner
	if !h.isAdmin(ctx, i.Member) {
		respondError(s, i, "❌ Hanya admin yang bisa mengubah status reminder!")
		return
	}

	options := parseOptions(i.ApplicationCommandData().Options)
	enabled := options["enabled"].BoolValue()

	if err := h.reminderSvc.ToggleReminder(ctx, i.GuildID, enabled); err != nil {
		respondError(s, i, fmt.Sprintf("❌ Gagal mengubah status: %v", err))
		return
	}

	status := "dinonaktifkan"
	emoji := "🔴"
	if enabled {
		status = "diaktifkan"
		emoji = "✅"
	}

	response := fmt.Sprintf("%s Reminder berhasil **%s**!", emoji, status)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
}

func (h *ReminderHandler) HandleReminderStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	config, err := h.reminderSvc.GetReminderConfig(ctx, i.GuildID)
	if err != nil || config == nil {
		respondError(s, i, "❌ Konfigurasi reminder belum dibuat! Gunakan `/reminder-setup` terlebih dahulu.")
		return
	}

	statusEmoji := "✅"
	statusText := "Aktif"
	if !config.IsEnabled {
		statusEmoji = "🔴"
		statusText = "Nonaktif"
	}

	response := fmt.Sprintf(
		"⚙️ **Status Konfigurasi Reminder**\n\n"+
			"📍 Channel: <#%s>\n"+
			"⏰ Waktu: %s\n"+
			"📅 Hari sebelum: %d hari\n"+
			"%s Status: **%s**\n"+
			"🔄 Terakhir diupdate: %s",
		config.ChannelID,
		config.ReminderTime.Format("15:04"),
		config.DaysBefore,
		statusEmoji,
		statusText,
		config.UpdatedAt.Format("02 Jan 2006 15:04"),
	)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
}

func (h *ReminderHandler) HandleReminderTest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	// Cek apakah user adalah admin atau owner
	if !h.isAdmin(ctx, i.Member) {
		respondError(s, i, "❌ Hanya admin yang bisa menjalankan test reminder!")
		return
	}

	// Defer response
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Jalankan check reminder
	if err := h.reminderSvc.CheckAndSendReminders(ctx); err != nil {
		editError(s, i, fmt.Sprintf("❌ Gagal menjalankan test reminder: %v", err))
		return
	}

	response := "✅ Test reminder berhasil dijalankan! Cek channel reminder untuk melihat hasilnya."
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &response,
	})
}

// Helper functions

func (h *ReminderHandler) isAdmin(ctx context.Context, member *discordgo.Member) bool {
	// Cek apakah user adalah owner
	if h.authSvc.IsOwner(member.User.ID) {
		return true
	}

	// Cek apakah user memiliki permission Administrator
	permissions := int64(member.Permissions)
	return (permissions & discordgo.PermissionAdministrator) == discordgo.PermissionAdministrator
}

func parseOptions(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	return optionMap
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func editError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &message,
	})
}

func ptr(f float64) *float64 {
	return &f
}
