package botcomm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSharedRecoveryContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/recovery.json")
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Messages    []*Message `json:"messages"`
		DuplicateID string     `json:"duplicateID"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}

	b := newTestBot()
	b.Messages = make(chan *Message, 256)
	b.Commands = make(chan *Message, 256)
	b.commands["/help"] = &Command{Value: "/help"}
	reads := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/session":
			_ = json.NewEncoder(w).Encode([]Session{{ID: "s", UnreadCount: 205}})
		case strings.HasSuffix(r.URL.Path, "/unread"):
			if r.URL.Query().Get("order") != "asc" {
				t.Error("not recovering oldest first")
			}

			start := 0
			if id := r.URL.Query().Get("afterID"); id != "" {
				for i, m := range fixture.Messages {
					if m.ID == id {
						start = i + 1
					}
				}

				if r.URL.Query().Get("afterTS") == "" {
					t.Error("missing cursor timestamp")
				}
			}

			_ = json.NewEncoder(w).Encode(fixture.Messages[start:min(start+100, len(fixture.Messages))])
		case strings.HasSuffix(r.URL.Path, "/read"):
			if len(b.Messages)+len(b.Commands) <= len(reads) {
				t.Error("acknowledged before enqueuing")
			}

			parts := strings.Split(r.URL.Path, "/")
			reads = append(reads, parts[len(parts)-2])
			w.WriteHeader(200)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	b.host = srv.URL
	b.httpClient = srv.Client()
	b.tk = &token{AccessToken: "test"}
	b.backfill(context.Background())
	if len(reads) != 205 {
		t.Fatalf("read %d messages", len(reads))
	}

	for i, id := range reads {
		if id != fmt.Sprintf("m%03d", i) {
			t.Fatalf("out of order: %d %s", i, id)
		}
	}

	if len(b.Commands) != 1 {
		t.Fatalf("commands=%d", len(b.Commands))
	}

	duplicate, _ := json.Marshal(map[string]any{"type": "message", "data": fixture.Messages[204]})
	if b.dispatch(duplicate) != nil {
		t.Fatal("backfill/SSE duplicate routed twice")
	}

	if len(b.Messages) != 204 {
		t.Fatalf("messages=%d", len(b.Messages))
	}
}

func TestCancelledDeliveryIsNotAcknowledged(t *testing.T) {
	b := newTestBot()
	b.Messages = make(chan *Message)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if msg := b.dispatchContext(ctx, []byte(`{"type":"message","data":{"id":"m","sessionID":"s","content":"hi"}}`)); msg != nil {
		t.Fatal("returned message for acknowledgement without accepting it")
	}
}
