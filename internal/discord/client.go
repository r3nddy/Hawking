package discord

import (


	"github.com/bwmarrin/discordgo"
	"hawking-bot/internal/services"
)

type Client struct {
	Session *discordgo.Session
	music   *services.MusicService
	Router  *Router
}

func NewClient(token string, router *Router) (*Client, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuilds

	client := &Client{
		Session: dg,
		Router:  router,
	}

	client.Session.AddHandler(client.handleMessage)
	client.Session.AddHandler(client.handleSlashCommand)
	client.Session.AddHandler(client.handleVoiceStateUpdate)
	client.Session.AddHandler(client.handleVoiceServerUpdate)

	return client, nil
}

func (c *Client) Connect(music *services.MusicService) error {
	c.music = music

	if err := c.Session.Open(); err != nil {
		return err
	}

	if err := c.Router.RegisterCommands(c.Session); err != nil {
		return err
	}

	return nil
}

func (c *Client) Close() {
	if c.Session != nil {
		c.Session.Close()
	}
}

func (c *Client) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "ping" {
		s.ChannelMessageSend(m.ChannelID, "pong!")
	}
}

func (c *Client) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	c.Router.HandleInteraction(s, i)
}

func (c *Client) handleVoiceStateUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	if c.music != nil {
		c.music.NotifyVoiceState(e.GuildID)
	}
}

func (c *Client) handleVoiceServerUpdate(s *discordgo.Session, e *discordgo.VoiceServerUpdate) {
	if c.music != nil {
		c.music.NotifyVoiceServer(e.GuildID)
	}
}
