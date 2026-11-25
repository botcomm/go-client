package botcomm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

// backfill loads the sessions and re-delivers messages that arrived while the
// bot was offline, oldest first, confirming each as read.
func (b *Bot) backfill(ctx context.Context) {
	sessions, err := b.Sessions(ctx)
	if err != nil {
		log.Printf("[ERROR] failed to get sessions: %v", err)
		return
	}

	delivered := map[string]struct{}{}
	for _, session := range sessions {
		if session.UnreadCount == 0 {
			continue
		}

		var after *Message
		for ctx.Err() == nil && !b.stopped.Load() {
			unread, err := b.loadUnread(ctx, session.ID, after)
			if err != nil {
				log.Printf("[ERROR] failed to load unread messages: %v", err)
				break
			}

			for _, msg := range unread {
				if _, duplicate := delivered[msg.ID]; duplicate {
					continue
				}

				if !b.route(ctx, msg) {
					return
				}

				delivered[msg.ID] = struct{}{}
				b.confirmRead(ctx, msg.SessionID, msg.ID)
			}

			if len(unread) < 100 {
				break
			}

			last := unread[len(unread)-1]
			if after != nil && last.ID == after.ID {
				break
			}

			after = last
		}
	}

	// remember what was just delivered so dispatch can drop a duplicate that
	// arrived over the stream while the backfill was running
	b.mu.Lock()
	b.backfilled = delivered
	b.mu.Unlock()
}

func (b *Bot) loadUnread(ctx context.Context, sessionID string, after *Message) ([]*Message, error) {
	messages := []*Message{}
	query := url.Values{"order": {"asc"}}
	if after != nil {
		query.Set("afterID", after.ID)
		query.Set("afterTS", after.CreateTS.Format(time.RFC3339Nano))
	}

	if err := b.do(ctx, http.MethodGet, fmt.Sprintf("/session/%s/message/unread?%s", sessionID, query.Encode()), nil, &messages); err != nil {
		return nil, fmt.Errorf("failed to get unread messages: %w", err)
	}

	return messages, nil
}

func (b *Bot) confirmRead(ctx context.Context, sessionID, messageID string) {
	if err := b.do(ctx, http.MethodPost, fmt.Sprintf("/session/%s/message/%s/read", sessionID, messageID), nil, nil); err != nil {
		log.Printf("[ERROR] failed to confirm message read: %v", err)
	}
}
