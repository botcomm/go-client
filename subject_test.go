package botcomm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendOptionsAndLegacySend(t *testing.T) {
	requests := []map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session/s1/message" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		requests = append(requests, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "m1", "sessionID": "s1", "content": body["content"], "subject": body["subject"]})
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
	msg, err := b.SendWithOptions(context.Background(), "s1", "News", SendOptions{Subject: "@dailynews", Silent: true})
	if err != nil || msg.Subject != "@dailynews" || msg.Content != "News" {
		t.Fatalf("msg=%+v, err=%v", msg, err)
	}

	if requests[0]["subject"] != "@dailynews" || requests[0]["silent"] != true || requests[0]["ts"] == "" {
		t.Fatalf("subject request lost fields: %+v", requests[0])
	}

	if _, err := b.Send(context.Background(), "s1", "Legacy"); err != nil {
		t.Fatal(err)
	}

	if _, ok := requests[1]["subject"]; ok || requests[1]["content"] != "Legacy" || requests[1]["silent"] != false {
		t.Fatalf("legacy send changed: %+v", requests[1])
	}
}

func TestCardHelpersPreserveLegacyMessageTypes(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 3 {
			if r.Method != http.MethodGet {
				t.Error("expected GET")
			}

			_, _ = w.Write([]byte(`{"id":"answer","fields":{"targets":["one"]}}`))

			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		card := body["card"].(map[string]any)
		if calls == 1 && card["kind"] != "plan" {
			t.Errorf("unexpected card kind: %v", card["kind"])
		}

		if calls == 2 && card["revision"] != float64(1) {
			t.Error("missing revision")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"id": "plan", "card": card})
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
	ctx := context.Background()
	plan, err := b.SendCard(ctx, "s1", AgentCard{Kind: CardKindPlan, Title: "Deploy", Summary: "Production"})
	if err != nil || plan.Card.Kind != CardKindPlan {
		t.Fatalf("%+v %v", plan, err)
	}

	if _, err := b.UpdateCard(ctx, "s1", plan.ID, CardUpdate{Revision: 1, Status: "running"}); err != nil {
		t.Fatal(err)
	}

	answer, err := b.GetCardMessage(ctx, "s1", "answer")
	if err != nil || answer.Fields["targets"].([]any)[0] != "one" {
		t.Fatalf("%+v %v", answer, err)
	}
}

func TestUnifiedMessageIncludesStructuredEvents(t *testing.T) {
	var message Message
	if err := json.Unmarshal([]byte(`{"id":"answer","replyTo":"form","value":"submit","fields":{"topic":"Weather","targets":["one"]},"card":{"kind":"form","revision":2}}`), &message); err != nil {
		t.Fatal(err)
	}

	if message.Fields["topic"] != "Weather" || message.Card.Revision != 2 {
		t.Fatalf("lost structured message: %+v", message)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input MessageInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}

		if len(input.Actions) != 1 || input.Actions[0].Value != "weather" {
			t.Errorf("lost actions: %+v", input)
		}

		_ = json.NewEncoder(w).Encode(Message{ID: "choice", Content: input.Content})
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
	if _, err := b.SendMessage(context.Background(), "s1", MessageInput{Content: "Choose", Actions: []Action{{Label: "Weather", Value: "weather"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataSendEditAndBroadcast(t *testing.T) {
	var bodies []map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		bodies = append(bodies, body)
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	b := &Bot{ID: "bot", host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
	ctx := context.Background()
	metadata := json.RawMessage(`{"debug":[true,null,1.5]}`)
	message, err := b.SendWithOptions(ctx, "s", "Answer", SendOptions{Metadata: metadata})
	if err != nil || string(message.Metadata) != string(metadata) {
		t.Fatalf("send: %+v %v", message, err)
	}

	broadcast, err := b.BroadcastWithOptions(ctx, "Answer", SendOptions{Metadata: metadata})
	if err != nil || string(broadcast.Metadata) != string(metadata) {
		t.Fatalf("broadcast: %+v %v", broadcast, err)
	}

	for _, value := range []json.RawMessage{nil, metadata, json.RawMessage(`null`)} {
		if _, err := b.EditWithOptions(ctx, "s", "m", EditOptions{Metadata: value}); err != nil {
			t.Fatal(err)
		}

		if string(bodies[len(bodies)-1]["metadata"]) != string(value) {
			t.Fatal("metadata edit semantics changed")
		}
	}
}

func TestStreamAndReactionHelpers(t *testing.T) {
	var path string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		_, _ = w.Write([]byte(`{"id":"m","stream":{"status":"active","sequence":1},"reactions":{"user":"❤️"},"reactionRevision":2}`))
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}, Reactions: make(chan *Message, 1)}
	ctx := context.Background()
	message, err := b.StartStream(ctx, "s")
	if err != nil || body["stream"] != true || message.ReactionRevision != 2 {
		t.Fatalf("start: %+v %v", message, err)
	}

	if _, err := b.UpdateStream(ctx, "s", "m", StreamInput{Content: "Hello", Sequence: 1, Status: "active"}); err != nil || path != "/session/s/message/m/stream" || body["sequence"] != float64(1) {
		t.Fatalf("stream: %s %+v %v", path, body, err)
	}

	if _, err := b.React(ctx, "s", "m", ""); err != nil || path != "/session/s/message/m/reaction" || body["emoji"] != "" {
		t.Fatalf("reaction: %s %+v %v", path, body, err)
	}

	if msg := b.dispatch([]byte(`{"type":"reaction","data":{"id":"m","reactions":{"user":"❤️"}}}`)); msg != nil {
		t.Fatal("reaction routed as message")
	}

	select {
	case reaction := <-b.Reactions:
		if reaction.Reactions["user"] != "❤️" {
			t.Fatalf("reaction: %+v", reaction)
		}
	default:
		t.Fatal("missing reaction event")
	}
}
