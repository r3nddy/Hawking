package discord

import (
	"log"
	"github.com/bwmarrin/discordgo"
)

type CommandHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

type Router struct {
	commands map[string]CommandHandler
	components map[string]CommandHandler
	appCommands []*discordgo.ApplicationCommand
}

func NewRouter() *Router {
	return &Router{
		commands: make(map[string]CommandHandler),
		components: make(map[string]CommandHandler),
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
	for _, v := range r.appCommands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", v)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", v.Name, err)
		}
	}
	log.Println("Slash commands berhasil didaftarkan")
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
