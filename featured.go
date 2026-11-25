package botcomm

import (
	"context"
	"fmt"
	"net/http"
)

// FeaturedBot is a public bot in the operator's curated list.
type FeaturedBot struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}

// FeaturedBots returns the curated public bots in the operator's order.
func (b *Bot) FeaturedBots(ctx context.Context) ([]*FeaturedBot, error) {
	list := []*FeaturedBot{}
	if err := b.do(ctx, http.MethodGet, "/bot/featured", nil, &list); err != nil {
		return nil, fmt.Errorf("failed to get featured bots: %w", err)
	}

	return list, nil
}
