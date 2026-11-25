package botcomm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCardTaskStatuses(t *testing.T) {
	for _, status := range []CardTaskStatus{
		CardTaskStatusQueued, CardTaskStatusRunning, CardTaskStatusWaiting,
		CardTaskStatusCompleted, CardTaskStatusFailed, CardTaskStatusCancelled,
		"", "unknown",
	} {
		for _, method := range []string{"SendMessage", "UpdateMessage", "SendCard", "UpdateCard"} {
			t.Run(method+"/"+string(status), func(t *testing.T) {
				requests := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					var body MessageInput
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}

					if body.Card == nil || len(body.Card.Tasks) != 2 || body.Card.Tasks[1].Status != status {
						t.Errorf("unexpected card: %+v", body.Card)
					}

					_ = json.NewEncoder(w).Encode(Message{Card: body.Card})
				}))
				defer srv.Close()
				b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
				tasks := []CardTask{
					{ID: "first", Title: "First", Status: CardTaskStatusCompleted},
					{ID: "review", Title: "Review changes", Status: status},
				}
				card := AgentCard{Kind: "plan", Title: "Review", Tasks: tasks}
				change := CardUpdate{Revision: 1, Status: "running", Tasks: &tasks}
				ctx := context.Background()
				var err error
				switch method {
				case "SendMessage":
					_, err = b.SendMessage(ctx, "s1", MessageInput{Card: &card})
				case "UpdateMessage":
					_, err = b.UpdateMessage(ctx, "s1", "m1", MessageUpdate{Card: &change})
				case "SendCard":
					_, err = b.SendCard(ctx, "s1", card)
				case "UpdateCard":
					_, err = b.UpdateCard(ctx, "s1", "m1", change)
				}

				if status == "" || status == "unknown" {
					want := fmt.Sprintf("invalid task status %q for task 1 (%q)", status, "review")
					if err == nil || err.Error() != want || requests != 0 {
						t.Fatalf("err=%v, requests=%d; want %q without a request", err, requests, want)
					}
				} else if err != nil || requests != 1 {
					t.Fatalf("err=%v, requests=%d; want one successful request", err, requests)
				}
			})
		}
	}
}

func TestCardTasksOmittedAndCleared(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
	ctx := context.Background()
	if _, err := b.SendMessage(ctx, "s1", MessageInput{Content: "Hello"}); err != nil {
		t.Fatal(err)
	}

	if _, err := b.UpdateMessage(ctx, "s1", "m1", MessageUpdate{}); err != nil {
		t.Fatal(err)
	}

	if _, err := b.SendCard(ctx, "s1", AgentCard{Kind: "plan", Title: "Review"}); err != nil {
		t.Fatal(err)
	}

	if _, err := b.UpdateCard(ctx, "s1", "m1", CardUpdate{Revision: 1, Status: "running"}); err != nil {
		t.Fatal(err)
	}

	tasks := []CardTask{}
	message, err := b.UpdateCard(ctx, "s1", "m1", CardUpdate{Revision: 1, Status: "running", Tasks: &tasks})
	if err != nil {
		t.Fatal(err)
	}

	if message.Card.Tasks == nil || len(message.Card.Tasks) != 0 {
		t.Fatalf("expected an empty task list, got %+v", message.Card.Tasks)
	}
}

func TestUpdateCardMetadata(t *testing.T) {
	for _, metadata := range []json.RawMessage{nil, json.RawMessage(`{"model":"example"}`), json.RawMessage(`{}`), json.RawMessage(`null`)} {
		t.Run(string(metadata), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}

				if string(body["metadata"]) != string(metadata) {
					t.Errorf("metadata = %s, want %s", body["metadata"], metadata)
				}

				var card map[string]json.RawMessage
				if err := json.Unmarshal(body["card"], &card); err != nil {
					t.Error(err)
					return
				}

				if _, ok := card["metadata"]; ok {
					t.Error("metadata must be on the message, not inside the card")
				}

				if string(card["revision"]) != "1" || string(card["status"]) != `"running"` {
					t.Errorf("unexpected card update: %s", body["card"])
				}

				_ = json.NewEncoder(w).Encode(body)
			}))
			defer srv.Close()
			b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
			message, err := b.UpdateCard(context.Background(), "s1", "m1", CardUpdate{Revision: 1, Status: "running", Metadata: metadata})
			if err != nil {
				t.Fatal(err)
			}

			if string(message.Metadata) != string(metadata) {
				t.Errorf("response metadata = %s, want %s", message.Metadata, metadata)
			}
		})
	}
}
