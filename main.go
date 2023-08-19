package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/caarlos0/env/v9"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
)

var chatCommand = &discordgo.ApplicationCommand{
	Name:        "chat",
	Description: "The prompt for the conversation",
	Type:        discordgo.ChatApplicationCommand,
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "dialogue_prompt",
			Description: "The prompt for the conversation, this will set the behavior of the bot",
			Required:    true,
		},
	},
}

type Bot struct {
	cfg     config
	llm     *openai.Chat
	discord *discordgo.Session
}

func main() {
	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
	log.Println("config load")

	llm, err := openai.NewChat(openai.WithModel(cfg.Model), openai.WithBaseURL(cfg.OpenaiAPIBase), openai.WithToken(cfg.OpenaiAPIKey))
	if err != nil {
		log.Fatal(err)
	}

	s, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		log.Fatalf("error creating session: %v", err)
	}

	_, err = s.ApplicationCommandCreate(cfg.ApplicationID, "", chatCommand)
	if err != nil {
		log.Fatalf("Cannot create command: %v", err)
	}

	b := &Bot{
		cfg:     cfg,
		llm:     llm,
		discord: s,
	}
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("now ready")
	})
	s.AddHandler(b.onMessage())
	s.AddHandler(b.onChatCommand())

	s.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)
	err = s.Open()
	if err != nil {
		log.Fatalf("error opening connection: %v", err)
	}
	defer func(s *discordgo.Session) {
		err := s.Close()
		if err != nil {
			log.Println("error closing session:", err)
		}
	}(s)

	log.Println("bot is now running")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)
	<-stop
}

// onChatCommand is the handler for the chat command
func (b *Bot) onChatCommand() func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		channel, err := s.State.Channel(i.ChannelID)
		if err != nil {
			log.Println("error fetching channel:", err)
			return
		}

		// Check if the channel is a text channel
		if channel.Type != discordgo.ChannelTypeGuildText {
			return
		}

		name, err := getNickname(s, i.GuildID, i.Member.User.ID)
		if err != nil {
			log.Println("error getting nickname:", err)
			return
		}

		prompt := i.ApplicationCommandData().Options[0].StringValue()
		embed := &discordgo.MessageEmbed{
			Color:       0x4CAF50, // nice green
			Description: fmt.Sprintf("<@%s> wants to chat!\n Prompt: %s", i.Member.User.ID, prompt),
		}

		// Respond to the interaction with the embed in the original channel
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				AllowedMentions: &discordgo.MessageAllowedMentions{
					Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
				},
			},
		}, discordgo.WithRetryOnRatelimit(true), discordgo.WithContext(ctx))
		if err != nil {
			log.Println("error responding to interaction:", err)
			return
		}
		resMessage, err := s.InteractionResponse(i.Interaction, discordgo.WithRetryOnRatelimit(true), discordgo.WithContext(ctx))
		if err != nil {
			log.Println("error fetching interaction response:", err)
			return
		}

		_, err = s.MessageThreadStartComplex(i.ChannelID, resMessage.ID, &discordgo.ThreadStart{
			Name:                fmt.Sprintf("(%s) %s", name, b.cfg.ThreadName),
			AutoArchiveDuration: 60,
			Invitable:           false,
			RateLimitPerUser:    10,
			Type:                discordgo.ChannelTypeGuildPublicThread,
		}, discordgo.WithRetryOnRatelimit(true), discordgo.WithContext(ctx))
		if err != nil {
			log.Println("error creating thread from message:", err)
		}
	}
}

// onMessage is the handler for messages in a thread
func (b *Bot) onMessage() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // local generation can be slow with lots of tokens
		defer cancel()

		// Check if the message is from the bot
		if m.Author.ID == s.State.User.ID {
			return
		}

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			log.Println("error fetching channel:", err)
			return
		}

		// Check if the message is in a thread
		if !channel.IsThread() {
			return
		}

		// Check if the thread is owned by the bot
		if channel.OwnerID != s.State.User.ID {
			return
		}

		// Check if the thread is archived or locked
		if channel.ThreadMetadata.Archived || channel.ThreadMetadata.Locked {
			return
		}

		messages, err := s.ChannelMessages(m.ChannelID, b.cfg.MaxMessagesLength, "", "", "") // todo make smarter
		if err != nil {
			log.Println("error fetching messages:", err)
			return
		}

		completionMessages, err := b.getCompletionMessages(s, m, messages)
		if err != nil {
			log.Println("error getting completion messages:", err)
			return
		}

		resp, err := b.handleTypingAndCompletion(ctx, s, m, completionMessages)
		if err != nil {
			log.Println("error handling typing and completion:", err)
			return
		}
		_, err = s.ChannelMessageSend(m.ChannelID, resp, discordgo.WithContext(ctx))
		if err != nil {
			log.Println("error sending message:", err)
			return
		}
	}
}

// handleTypingAndCompletion handles typing and completion
func (b *Bot) handleTypingAndCompletion(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, completionMessages []schema.ChatMessage) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			err := s.ChannelTyping(m.ChannelID, discordgo.WithRetryOnRatelimit(true), discordgo.WithContext(ctx))
			if err != nil {
				log.Println("error typing:", err)
				return
			}
		}
	}()

	resp, err := b.llm.Call(
		ctx,
		completionMessages,
		llms.WithModel(b.cfg.Model),
		llms.WithTemperature(0.7),
	)
	if err != nil {
		return "", fmt.Errorf("error creating chat completion: %v", err)
	}

	return resp.Content, err
}

// getCompletionMessages returns a list of completion messages
func (b *Bot) getCompletionMessages(s *discordgo.Session, m *discordgo.MessageCreate, messages []*discordgo.Message) ([]schema.ChatMessage, error) {
	var completionMessages []schema.ChatMessage
	for _, message := range messages {
		name, err := getNickname(s, m.GuildID, message.Author.ID)
		if err != nil {
			log.Println("error getting nickname:", err)
			return nil, err
		}
		var cm schema.ChatMessage
		if message.Author.ID == s.State.User.ID && (message.ReferencedMessage != nil && len(message.ReferencedMessage.Embeds) > 0) {
			fullDescription := message.ReferencedMessage.Embeds[0].Description
			promptPrefix := "Prompt: "
			startIndex := strings.Index(fullDescription, promptPrefix)
			if startIndex == -1 {
				log.Println("Prompt prefix not found in embed description")
				return nil, fmt.Errorf("prompt prefix not found in embed description")
			}
			systemMessageContent := fullDescription[startIndex+len(promptPrefix):]
			cm = schema.SystemChatMessage{
				Content: fmt.Sprintf("%s\n%s", systemMessageContent, fmt.Sprintf(b.cfg.SystemPromptSuffix, name)),
			}
		} else if message.Author.ID == s.State.User.ID {
			cm = schema.AIChatMessage{
				Content: fmt.Sprintf("%s", message.Content),
			}
		} else {
			cm = schema.HumanChatMessage{
				Content: fmt.Sprintf("(%s) %s", name, message.Content),
			}
		}
		completionMessages = append([]schema.ChatMessage{cm}, completionMessages...)
	}
	return completionMessages, nil
}

// getNickname returns the nickname of the user
func getNickname(s *discordgo.Session, guildID string, userID string) (string, error) {
	member, err := s.GuildMember(guildID, userID)
	if err != nil {
		return "", err
	}

	if member.Nick != "" {
		return member.Nick, nil
	} else {
		return member.User.Username, nil
	}
}
