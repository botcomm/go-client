package botcomm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoRetriesAfter429(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Idempotency-Key") != "themis:issue:123" {
			t.Error("missing idempotency key")
		}

		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		_, _ = w.Write([]byte(`{"id":"m1","content":"hi"}`))
	}))
	defer srv.Close()

	b := newTestBot()
	b.host = srv.URL
	b.tk = &token{AccessToken: "tk"}
	start := time.Now()
	msg, err := b.SendMessage(context.Background(), "s1", MessageInput{Content: "hi", IdempotencyKey: "themis:issue:123"})
	if err != nil || msg.ID != "m1" || calls != 2 {
		t.Fatalf("msg=%+v err=%v calls=%d", msg, err, calls)
	}

	if time.Since(start) < time.Second {
		t.Fatal("did not wait for Retry-After")
	}
}

func TestRecentMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.RequestURI() != "/session/s1/message?limit=10" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}

		_, _ = w.Write([]byte(`{"count":14,"list":[{"id":"latest"},{"id":"older"}]}`))
	}))
	defer srv.Close()

	b := newTestBot()
	b.host = srv.URL
	b.tk = &token{AccessToken: "tk"}
	history, err := b.RecentMessages(context.Background(), "s1")
	if err != nil || len(history) != 2 || history[0].ID != "latest" || history[1].ID != "older" {
		t.Fatalf("history=%v err=%v", history, err)
	}
}
