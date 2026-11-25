package botcomm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Envelope is a single /updates event: a JSON message with a
// type ("message", "new_session", "read", "settings") and a type-specific payload.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// connect opens the /updates event stream with the current access token.
func (b *Bot) connect() error {
	req, err := http.NewRequest(http.MethodGet, b.host+"/updates", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+b.accessToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", req.URL, err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			return errUnauthorized
		}

		return fmt.Errorf("failed to connect to %s: status %d", req.URL, resp.StatusCode)
	}

	b.mu.Lock()
	b.conn = resp.Body
	b.mu.Unlock()

	return nil
}

// listen back-fills unread messages, reads events until the stream breaks
// and reconnects every reconnectDelay until ctx is cancelled or Stop is called.
func (b *Bot) listen(ctx context.Context) {
	for {
		b.backfill(ctx)
		if ctx.Err() != nil || b.stopped.Load() {
			return
		}

		b.mu.RLock()
		conn := b.conn
		b.mu.RUnlock()

		watchdog := time.AfterFunc(readWait, func() { _ = conn.Close() })
		r := bufio.NewReader(conn)
		var data []byte
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				_ = conn.Close()
				if ctx.Err() == nil && !b.stopped.Load() {
					log.Printf("[WARN] stream read failed: %v", err)
				}

				break
			}

			watchdog.Reset(readWait)

			line = bytes.TrimRight(line, "\r\n")
			switch {
			case len(line) == 0:
				if len(data) > 0 {
					if msg := b.dispatchContext(ctx, data); msg != nil {
						go b.confirmRead(ctx, msg.SessionID, msg.ID)
					}

					data = nil
				}
			case bytes.HasPrefix(line, []byte("data:")):
				if len(data) > 0 {
					data = append(data, '\n')
				}

				data = append(data, bytes.TrimPrefix(line[5:], []byte(" "))...)
			}
		}

		watchdog.Stop()

		if ctx.Err() != nil || b.stopped.Load() {
			return
		}

		for {
			if err := b.connect(); err != nil {
				log.Printf("[WARN] failed to reconnect: %v", err)
				if errors.Is(err, errUnauthorized) { // the token was revoked or the server's keys changed
					if tk, err := b.grant(ctx); err == nil {
						b.mu.Lock()
						b.tk = tk
						b.mu.Unlock()
					}
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(reconnectDelay):
				}

				if b.stopped.Load() {
					return
				}

				continue
			}

			log.Printf("[INFO] reconnected to the event stream")
			break
		}
	}
}

// dispatch parses one event and routes it: "message" envelopes go to the
// Messages or Commands channel (returned for read confirmation), "new_session"
// invokes the NewSession callback. Settings, read receipts and collection
// requests go to their respective channels.
func (b *Bot) dispatch(frame []byte) *Message {
	return b.dispatchContext(context.Background(), frame)
}

func (b *Bot) dispatchContext(ctx context.Context, frame []byte) *Message {
	var env Envelope
	if err := json.Unmarshal(frame, &env); err != nil {
		log.Printf("[WARN] failed to unmarshal envelope: %v", err)
		return nil
	}

	switch env.Type {
	case "message":
		msg := &Message{}
		if err := json.Unmarshal(env.Data, msg); err != nil {
			log.Printf("[WARN] failed to unmarshal message: %v", err)
			return nil
		}

		b.mu.Lock()
		_, dup := b.backfilled[msg.ID]
		delete(b.backfilled, msg.ID)
		b.mu.Unlock()
		if dup {
			return nil // already delivered (and confirmed read) by the backfill
		}

		if !b.route(ctx, msg) {
			return nil
		}

		return msg
	case "reaction":
		msg := &Message{}
		if json.Unmarshal(env.Data, msg) == nil {
			select {
			case b.Reactions <- msg:
			default:
			}
		}
	case "new_session":
		session := &Session{}
		if err := json.Unmarshal(env.Data, session); err != nil {
			log.Printf("[WARN] failed to unmarshal session: %v", err)
			return nil
		}

		if b.NewSession != nil {
			b.NewSession(session)
		}
	case "collection_request":
		var req CollectionRequest
		if err := json.Unmarshal(env.Data, &req); err != nil {
			return nil
		}

		select {
		case b.CollectionRequests <- &req:
		default:
		}
	case "settings":
		settings := &SessionSettings{}
		if err := json.Unmarshal(env.Data, settings); err != nil {
			log.Printf("[WARN] failed to unmarshal settings: %v", err)
			return nil
		}

		select {
		case b.Settings <- settings:
		default:
		}
	case "read":
		receipt := &ReadReceipt{}
		if err := json.Unmarshal(env.Data, receipt); err != nil {
			log.Printf("[WARN] failed to unmarshal read receipt: %v", err)
			return nil
		}

		select {
		case b.ReadReceipts <- receipt:
		default:
		}
	default:
		log.Printf("[WARN] unknown envelope type %q", env.Type)
	}

	return nil
}
