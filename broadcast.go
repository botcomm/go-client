package botcomm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Broadcast is a message fanned out to every session of the bot by the server.
type Broadcast struct {
	ID       string          `json:"id"`
	Content  string          `json:"content"`
	Subject  string          `json:"subject,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Silent   bool            `json:"silent"`
	Total    int             `json:"total"`
	Sent     int             `json:"sent"`
	Failed   int             `json:"failed"`
	Done     bool            `json:"done"`
	CreateTS time.Time       `json:"createTS"`
	DoneTS   *time.Time      `json:"doneTS,omitempty"`
}

// Broadcast sends content to every session of the bot as a server-side job.
// Poll BroadcastStatus for progress.
func (b *Bot) Broadcast(ctx context.Context, content string, silent ...bool) (*Broadcast, error) {
	return b.BroadcastWithOptions(ctx, content, SendOptions{Silent: len(silent) > 0 && silent[0]})
}

// BroadcastKeyboard is Broadcast with one-tap answer options on every copy.
func (b *Bot) BroadcastKeyboard(ctx context.Context, content string, options []KeyboardOption) (*Broadcast, error) {
	return b.BroadcastWithOptions(ctx, content, SendOptions{Keyboard: &Keyboard{Options: options}})
}

// BroadcastWithOptions includes an optional subject or keyboard on every copy.
func (b *Bot) BroadcastWithOptions(ctx context.Context, content string, options SendOptions) (*Broadcast, error) {
	req := struct {
		SendOptions
		Content string `json:"content"`
	}{SendOptions: options, Content: content}
	out := &Broadcast{}
	if err := b.do(ctx, http.MethodPost, fmt.Sprintf("/bot/%s/broadcast", b.ID), req, out); err != nil {
		return nil, fmt.Errorf("failed to broadcast: %w", err)
	}

	return out, nil
}

// BroadcastStatus returns the progress of a broadcast job.
func (b *Bot) BroadcastStatus(ctx context.Context, id string) (*Broadcast, error) {
	out := &Broadcast{}
	if err := b.do(ctx, http.MethodGet, fmt.Sprintf("/bot/%s/broadcast/%s", b.ID, id), nil, out); err != nil {
		return nil, fmt.Errorf("failed to get broadcast: %w", err)
	}

	return out, nil
}
