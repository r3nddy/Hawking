package discord

import (
	"log"
	"github.com/bwmarrin/discordgo"
)

type CommandHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

type Router struct {
	commands    map[string]CommandHandler
	components  map[string]CommandHandler
	appCommands []*discordgo.ApplicationCommand
	guildID     string
}

func NewRouter(guildID string) *Router {
	return &Router{
		commands:   make(map[string]CommandHandler),
		components: make(map[string]CommandHandler),
		guildID:    guildID,
	}
}

func (r *Router) Register(cmd *discordgo.ApplicationCommand, handler CommandHandler) {
	r.commands[cmd.Name] = handler
	r.appCommands = append(r.appCommands, cmd)
}

func (r *Router) RegisterComponent(customIDPrefix string, handler CommandHandler) {
	r.components[customIDPrefix] = handler
}

func (r *Router) RegisterCommands(s *discordgo.Session) error {
	_, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, r.guildID, r.appCommands)
	if err != nil {
		log.Printf("Cannot overwrite commands: %v", err)
		return err
	}
	log.Println("Slash commands berhasil didaftarkan (Bulk Overwrite)")
	return nil
}

func (r *Router) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if h, ok := r.commands[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID
		for prefix, h := range r.components {
			if len(customID) >= len(prefix) && customID[:len(prefix)] == prefix {
				h(s, i)
				return
			}
		}
	}
}
