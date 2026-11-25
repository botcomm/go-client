package botcomm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// CommandInput describes a single text argument collected by the client.
type CommandInput struct {
	Type        string `json:"type"`
	Label       string `json:"label,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required"`
	MaxLength   int    `json:"maxLength,omitempty"`
}

// Command describes a bot command shown to users.
type Command struct {
	Input       *CommandInput `json:"input,omitempty"`
	Name        string        `json:"name"`
	Value       string        `json:"value"`
	Description string        `json:"description"`
}

// RegisterCommands publishes the bot commands to the API and routes incoming
// messages whose content equals a command value to the Commands channel.
func (b *Bot) RegisterCommands(ctx context.Context, commands []*Command) error {
	if commands == nil {
		commands = []*Command{}
	}

	req := struct {
		Commands []*Command `json:"commands"`
	}{
		Commands: commands,
	}
	if err := b.do(ctx, http.MethodPost, fmt.Sprintf("/bot/%s", b.ID), req, nil); err != nil {
		return fmt.Errorf("failed to set commands: %w", err)
	}

	b.mu.Lock()
	b.commands = make(map[string]*Command, len(commands))
	for _, cmd := range commands {
		b.commands[cmd.Value] = cmd
	}

	b.mu.Unlock()

	return nil
}

// SetInputMode configures user input. Publish commands successfully first.
func (b *Bot) SetInputMode(ctx context.Context, mode string) error {
	return b.do(ctx, http.MethodPost, fmt.Sprintf("/bot/%s", b.ID), map[string]string{"inputMode": mode}, nil)
}

// SplitCommand preserves the argument as one text value, including internal spaces.
func SplitCommand(content string) (string, string) {
	if i := strings.IndexAny(content, " \t\r\n"); i >= 0 {
		return content[:i], strings.Trim(content[i:], " \t\r\n")
	}

	return content, ""
}

// SetMessageAcceptor installs durable acceptance before delivery and read acknowledgment.
// Configure it before Run. Failed acceptance is retried until shutdown.
func (b *Bot) SetMessageAcceptor(accept func(context.Context, *Message) error) {
	b.acceptMessage = accept
}

// route delivers a message to the Commands channel when its content (or, for
// keyboard answers, its machine value) matches a registered command value, to
// the Messages channel otherwise.
func (b *Bot) route(ctx context.Context, msg *Message) bool {
	if b.acceptMessage != nil {
		for {
			if ctx.Err() != nil {
				return false
			}

			if err := b.acceptMessage(ctx, msg); err == nil {
				break
			} else {
				log.Printf("[WARN] durable message acceptance failed message_id=%q error=%v", msg.ID, err)
			}

			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
			}
		}
	}

	key := msg.Content
	if msg.Value != "" {
		key = msg.Value
	}

	b.mu.RLock()
	_, isCommand := b.commands[key]
	if b.ArgumentCommands && msg.Value == "" && msg.ReplyTo == "" {
		if isCommand {
			msg.Command = key
		} else {
			command, argument := SplitCommand(msg.Content)
			if c := b.commands[command]; c != nil && c.Input != nil {
				isCommand = true
				msg.Command, msg.Argument = command, argument
			}
		}
	}

	b.mu.RUnlock()

	channel := b.Messages
	if isCommand {
		channel = b.Commands
	}

	select {
	case <-ctx.Done():
		return false
	case channel <- msg:
		return true
	}
}
