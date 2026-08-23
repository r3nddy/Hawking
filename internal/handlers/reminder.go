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

func NewReminderHandler(reminderSvc *services.ReminderService, authSvc *services.AuthService, router *discord.Router) *ReminderHandler {
	h := &ReminderHandler{reminderSvc: reminderSvc, authSvc: authSvc}

	router.Register(&discordgo.ApplicationCommand{
		Name: "reminder-setup", Description: "Setup konfigurasi reminder untuk server ini (Admin only)",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Channel untuk mengirim reminder", Required: true, ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText}},
			{Type: discordgo.ApplicationCommandOptionString, Name: "waktu", Description: "Jam pengiriman reminder (format: HH:MM, contoh: 20:00)", Required: false},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "hari-sebelum", Description: "Berapa hari sebelum jadwal (default: 1)", Required: false, MinValue: ptr(0.0), MaxValue: 7},
		},
	}, h.HandleReminderSetup)
	router.Register(&discordgo.ApplicationCommand{
		Name: "reminder-toggle", Description: "Aktifkan/nonaktifkan reminder (Admin only)",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionBoolean, Name: "enabled", Description: "Aktifkan (true) atau nonaktifkan (false)", Required: true}},
	}, h.HandleReminderToggle)
	router.Register(&discordgo.ApplicationCommand{Name: "reminder-status", Description: "Lihat status konfigurasi reminder"}, h.HandleReminderStatus)
	router.Register(&discordgo.ApplicationCommand{Name: "reminder-test", Description: "Test reminder secara manual (Admin only)"}, h.HandleReminderTest)
	return h
}

func (h *ReminderHandler) HandleReminderSetup(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	if !h.isAdmin(i.Member) {
		respondError(s, i, "❌ Hanya admin yang bisa mengatur konfigurasi reminder!")
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource})

	options := parseOptions(i.ApplicationCommandData().Options)
	channelID := options["channel"].ChannelValue(s).ID
	reminderTime, daysBefore := "20:00:00", 1
	if waktu, ok := options["waktu"]; ok {
		parsed, err := time.Parse("15:04", waktu.StringValue())
		if err != nil {
			editError(s, i, "❌ Format waktu tidak valid! Gunakan format HH:MM (contoh: 20:00)")
			return
		}
		reminderTime = parsed.Format("15:04:05")
	}
	if hari, ok := options["hari-sebelum"]; ok {
		daysBefore = int(hari.IntValue())
	}
	parsedTime, err := time.Parse("15:04:05", reminderTime)
	if err != nil {
		editError(s, i, "❌ Error parsing waktu!")
		return
	}
	fullTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, time.Local)
	config := &models.ReminderConfig{GuildID: i.GuildID, ChannelID: channelID, ReminderTime: fullTime, IsEnabled: true, DaysBefore: daysBefore}
	if err := h.reminderSvc.SetupReminderConfig(ctx, config); err != nil {
		editError(s, i, fmt.Sprintf("❌ Gagal menyimpan konfigurasi: %v", err))
		return
	}
	response := fmt.Sprintf("✅ **Konfigurasi Reminder Berhasil!**\n\n📍 Channel: <#%s>\n⏰ Waktu: %s\n📅 Hari sebelum: %d hari\n✅ Status: Aktif\n\nReminder akan dikirim otomatis sesuai jadwal yang dikonfigurasi.", channelID, parsedTime.Format("15:04"), daysBefore)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &response})
}

func (h *ReminderHandler) HandleReminderToggle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	if !h.isAdmin(i.Member) {
		respondError(s, i, "❌ Hanya admin yang bisa mengubah status reminder!")
		return
	}
	enabled := parseOptions(i.ApplicationCommandData().Options)["enabled"].BoolValue()
	if err := h.reminderSvc.ToggleReminder(ctx, i.GuildID, enabled); err != nil {
		respondError(s, i, fmt.Sprintf("❌ Gagal mengubah status: %v", err))
		return
	}
	status, emoji := "dinonaktifkan", "🔴"
	if enabled {
		status, emoji = "diaktifkan", "✅"
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("%s Reminder berhasil **%s**!", emoji, status)}})
}

func (h *ReminderHandler) HandleReminderStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	config, err := h.reminderSvc.GetReminderConfig(context.Background(), i.GuildID)
	if err != nil || config == nil {
		respondError(s, i, "❌ Konfigurasi reminder belum dibuat! Gunakan `/reminder-setup` terlebih dahulu.")
		return
	}
	statusEmoji, statusText := "✅", "Aktif"
	if !config.IsEnabled {
		statusEmoji, statusText = "🔴", "Nonaktif"
	}
	response := fmt.Sprintf("⚙️ **Status Konfigurasi Reminder**\n\n📍 Channel: <#%s>\n⏰ Waktu: %s\n📅 Hari sebelum: %d hari\n%s Status: **%s**\n🔄 Terakhir diupdate: %s", config.ChannelID, config.ReminderTime.Format("15:04"), config.DaysBefore, statusEmoji, statusText, config.UpdatedAt.Format("02 Jan 2006 15:04"))
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: response}})
}

func (h *ReminderHandler) HandleReminderTest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isAdmin(i.Member) {
		respondError(s, i, "❌ Hanya admin yang bisa menjalankan test reminder!")
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource})
	result, err := h.reminderSvc.CheckAndSendReminders(context.Background())
	if err != nil {
		editError(s, i, fmt.Sprintf("❌ Reminder gagal dikirim: %v", err))
		return
	}
	response := formatReminderTestResponse(result)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &response})
}

func formatReminderTestResponse(result *services.ReminderCheckResult) string {
	if result == nil {
		return "⚠️ Reminder tidak menghasilkan status."
	}
	if result.ActiveConfigs == 0 {
		return "⚠️ Belum ada konfigurasi reminder aktif. Gunakan `/reminder-setup` terlebih dahulu."
	}
	if result.SchedulesFound == 0 {
		return "ℹ️ Tidak ada jadwal kuliah pada tanggal target reminder. Tidak ada pesan yang dikirim."
	}
	if result.MessagesSent == 0 && result.Skipped > 0 {
		return "ℹ️ Reminder untuk jadwal target sudah pernah dikirim. Tidak ada pesan duplikat."
	}
	return fmt.Sprintf("✅ %d pesan reminder berhasil dikirim. Jadwal ditemukan: %d.", result.MessagesSent, result.SchedulesFound)
}

func (h *ReminderHandler) isAdmin(member *discordgo.Member) bool {
	if h.authSvc.IsOwner(member.User.ID) {
		return true
	}
	permissions := int64(member.Permissions)
	return permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator
}

func parseOptions(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	return optionMap
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: message, Flags: discordgo.MessageFlagsEphemeral}})
}

func editError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &message})
}

func ptr(f float64) *float64 { return &f }
