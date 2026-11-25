package botcomm

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// SettingField is one entry of the bot's per-session settings schema, rendered
// by the app as a form control. Type is "toggle" ("true"/"false"), "select"
// (Options), "text" or "time" ("HH:MM"); values are always strings.
type SettingField struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Description string           `json:"description,omitempty"`
	Type        string           `json:"type"`
	Options     []KeyboardOption `json:"options,omitempty"`
	Default     string           `json:"default,omitempty"`
}

// SessionSettings are the effective settings of a session, defaults included.
type SessionSettings struct {
	SessionID string            `json:"sessionID"`
	Settings  map[string]string `json:"settings"`
}

// ReadReceipt identifies a message read by the other side of a session.
type ReadReceipt struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	By        string `json:"by,omitempty"`
}

// User is the account on the other side of a session.
type User struct {
	ID       string  `json:"id"`
	Name     *string `json:"name,omitempty"`
	Timezone *string `json:"timezone,omitempty"` // IANA zone, e.g. Europe/Kyiv
	Locale   *string `json:"locale,omitempty"`   // BCP 47 tag, e.g. uk-UA
}

// Session is a conversation between one account and the bot.
type Session struct {
	ID          string            `json:"id"`
	User        *User             `json:"user,omitempty"`
	UnreadCount int               `json:"unreadCount"`
	LastMessage *Message          `json:"lastMessage,omitempty"`
	Settings    map[string]string `json:"settings,omitempty"` // effective values of the registered settings
	Muted       bool              `json:"muted"`              // the user receives no push for this session
	CreateTS    time.Time         `json:"createTS"`
}

// RegisterSettings publishes the per-session settings schema (up to 20 fields)
// the app renders for users. Every Session then carries Settings with defaults
// filled in, and changes arrive on the Settings channel.
func (b *Bot) RegisterSettings(ctx context.Context, fields []SettingField) error {
	req := struct {
		Settings []SettingField `json:"settings"`
	}{Settings: fields}
	if err := b.do(ctx, http.MethodPost, fmt.Sprintf("/bot/%s", b.ID), req, nil); err != nil {
		return fmt.Errorf("failed to set settings: %w", err)
	}

	return nil
}

// SetSettings changes settings of a session on the user's behalf and returns
// the effective values.
func (b *Bot) SetSettings(ctx context.Context, sessionID string, values map[string]string) (*SessionSettings, error) {
	out := &SessionSettings{}
	if err := b.do(ctx, http.MethodPost, fmt.Sprintf("/session/%s/settings", sessionID), values, out); err != nil {
		return nil, fmt.Errorf("failed to set session settings: %w", err)
	}

	return out, nil
}

// Sessions fetches all sessions of the bot.
func (b *Bot) Sessions(ctx context.Context) ([]*Session, error) {
	sessions := []*Session{}
	if err := b.do(ctx, http.MethodGet, "/session", nil, &sessions); err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	return sessions, nil
}
