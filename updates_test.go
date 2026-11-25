package botcomm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDispatchMessageEnvelope(t *testing.T) {
	b := newTestBot()

	frame := []byte(`{"type":"message","data":{"id":"m1","sessionID":"s1","authorID":"a1",` +
		`"content":"hello","silent":false,"createTS":"2026-08-31T12:00:00Z"}}`)

	msg := b.dispatch(frame)
	if msg == nil {
		t.Fatal("expected dispatch to return the routed message")
	}

	select {
	case got := <-b.Messages:
		if got.ID != "m1" || got.SessionID != "s1" || got.AuthorID != "a1" || got.Content != "hello" {
			t.Fatalf("unexpected message decoded: %+v", got)
		}

		if !got.CreateTS.Equal(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)) {
			t.Fatalf("unexpected createTS: %v", got.CreateTS)
		}
	default:
		t.Fatal("expected the message on the Messages channel")
	}

	if len(b.Commands) != 0 {
		t.Fatal("non-command message must not reach the Commands channel")
	}
}

func TestDispatchNewSession(t *testing.T) {
	b := newTestBot()

	var got *Session
	b.NewSession = func(session *Session) {
		got = session
	}

	frame := []byte(`{"type":"new_session","data":{"id":"s9","unreadCount":0,"createTS":"2026-08-31T12:00:00Z"}}`)
	if b.dispatch(frame) != nil {
		t.Fatal("new_session must not return a message")
	}

	if got == nil || got.ID != "s9" {
		t.Fatalf("expected NewSession callback with session s9, got %+v", got)
	}
}

func TestDispatchIgnoresOtherFrames(t *testing.T) {
	b := newTestBot()

	if b.dispatch([]byte(`{"type":"read","data":{"sessionID":"s1","messageID":"m1"}}`)) != nil {
		t.Fatal("read envelope must not return a message")
	}

	if b.dispatch([]byte(`{"type":"something_else","data":{}}`)) != nil {
		t.Fatal("unknown envelope must not return a message")
	}

	if b.dispatch([]byte(`not json`)) != nil {
		t.Fatal("invalid frame must not return a message")
	}

	if len(b.Messages) != 0 || len(b.Commands) != 0 {
		t.Fatal("no messages must be routed for non-message frames")
	}
}

func TestListenReadsEventStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_, _ = w.Write([]byte(`[]`))
		case "/updates":
			if r.Header.Get("Authorization") != "Bearer tk" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(": ping\n\ndata: {\"type\":\"message\",\"data\":{\"id\":\"m1\",\"sessionID\":\"s1\",\"content\":\"hi\"}}\n\n"))
		}
	}))
	defer srv.Close()

	b := newTestBot()
	b.host = srv.URL
	b.tk = &token{AccessToken: "tk"}
	if err := b.connect(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.listen(ctx)

	select {
	case got := <-b.Messages:
		if got.ID != "m1" || got.Content != "hi" {
			t.Fatalf("unexpected message: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no message from the event stream")
	}

	b.Stop()
}

func TestSettingsEnvelope(t *testing.T) {
	b := newTestBot()
	b.Settings = make(chan *SessionSettings, 1)
	if b.dispatch([]byte(`{"type":"settings","data":{"sessionID":"s1","settings":{"shout":"true"}}}`)) != nil {
		t.Fatal("settings envelope must not return a message")
	}

	select {
	case s := <-b.Settings:
		if s.SessionID != "s1" || s.Settings["shout"] != "true" {
			t.Fatalf("settings = %+v", s)
		}
	default:
		t.Fatal("settings not delivered")
	}

	b.dispatch([]byte(`{"type":"settings","data":{"sessionID":"s1","settings":{}}}`))
	b.dispatch([]byte(`{"type":"settings","data":{"sessionID":"s1","settings":{}}}`)) // full channel: dropped, no block
}
