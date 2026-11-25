package botcomm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestRunOptions(t *testing.T) {
	commands := []*Command{{Name: "Help", Value: "/help"}}
	sessions := make(chan *Session, 1)
	opts := &Opts{}
	if opts.SetCommands(commands).
		SetSettings([]SettingField{{Key: "enabled", Label: "Enabled", Type: "toggle"}}).
		SetCollections([]CollectionField{{Key: "tasks", Label: "Tasks", MaxItems: 10}}).
		SetInputMode("commands").
		SetOnlineStatusVisible(true).
		SetArgumentCommands(true).
		SetNewSession(func(session *Session) { sessions <- session }) != opts {
		t.Fatal("setters must return the same options pointer")
	}

	for _, tc := range []struct {
		name   string
		opts   []*Opts
		legacy bool
		want   []string
	}{
		{name: "omitted"},
		{name: "nil", opts: []*Opts{nil}},
		{name: "empty", opts: []*Opts{{}}},
		{
			name: "configured", opts: []*Opts{opts},
			want: []string{
				`{"commands":[{"name":"Help","value":"/help","description":""}]}`,
				`{"settings":[{"key":"enabled","label":"Enabled","type":"toggle"}]}`,
				`{"collections":[{"key":"tasks","label":"Tasks","maxItems":10}]}`,
				`{"onlineStatusVisible":true}`,
				`{"inputMode":"commands"}`,
			},
		},
		{
			name: "clear", opts: []*Opts{(&Opts{}).
				SetCommands([]*Command{}).
				SetSettings([]SettingField{}).
				SetCollections([]CollectionField{}).
				SetOnlineStatusVisible(false).
				SetInputMode("messages")},
			want: []string{
				`{"commands":[]}`, `{"settings":[]}`, `{"collections":[]}`,
				`{"onlineStatusVisible":false}`, `{"inputMode":"messages"}`,
			},
		},
		{
			name: "legacy", legacy: true,
			want: []string{
				`{"commands":[{"name":"Help","value":"/help","description":""}]}`,
				`{"inputMode":"commands"}`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan string, 8)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					_, _ = w.Write([]byte(`{"access_token":"eyJhbGciOiJub25lIn0.eyJzdWIiOiJib3QtMSJ9.sig","expires_in":3600}`))
				case "/bot/bot-1":
					body, _ := io.ReadAll(r.Body)
					requests <- string(body)
				case "/updates":
					if len(requests) != len(tc.want) {
						t.Error("connected before completing setup")
					}

					w.Header().Set("Content-Type", "text/event-stream")
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				case "/session":
					_, _ = w.Write([]byte(`[]`))
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			b := &Bot{host: server.URL, authHost: server.URL, ClientID: "id", ClientSecret: "secret"}
			defer b.Stop()
			if tc.legacy {
				b.StartupCommands, b.InputMode = commands, "commands"
			}

			if err := b.Run(context.Background(), tc.opts...); err != nil {
				t.Fatal(err)
			}

			if len(requests) != len(tc.want) {
				t.Fatalf("got %d setup requests, want %d", len(requests), len(tc.want))
			}

			for _, want := range tc.want {
				var gotBody, wantBody map[string]any
				if err := json.Unmarshal([]byte(<-requests), &gotBody); err != nil {
					t.Fatal(err)
				}

				if err := json.Unmarshal([]byte(want), &wantBody); err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(gotBody, wantBody) {
					t.Fatalf("setup request = %v, want %v", gotBody, wantBody)
				}
			}

			if tc.name == "configured" {
				if !b.ArgumentCommands {
					t.Fatal("argument routing was not enabled")
				}

				b.dispatch([]byte(`{"type":"message","data":{"content":"/help"}}`))
				if len(b.Commands) != 1 {
					t.Fatal("startup commands were not installed in the router")
				}

				b.dispatch([]byte(`{"type":"new_session","data":{"id":"new"}}`))
				select {
				case session := <-sessions:
					if session.ID != "new" {
						t.Fatalf("unexpected session: %+v", session)
					}
				default:
					t.Fatal("new session callback was not installed")
				}
			}
		})
	}
}

func TestRunOptionsFailure(t *testing.T) {
	for _, field := range []string{"commands", "settings", "collections", "onlineStatusVisible", "inputMode"} {
		t.Run(field, func(t *testing.T) {
			failed := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					_, _ = w.Write([]byte(`{"access_token":"eyJhbGciOiJub25lIn0.eyJzdWIiOiJib3QtMSJ9.sig","expires_in":3600}`))
					return
				}

				if failed || r.URL.Path != "/bot/bot-1" {
					t.Errorf("unexpected request after setup failure: %s", r.URL.Path)
				}

				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), `"`+field+`"`) {
					failed = true
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			b := &Bot{host: server.URL, authHost: server.URL}
			defer b.Stop()
			opts := (&Opts{}).SetCommands([]*Command{}).
				SetSettings([]SettingField{}).
				SetCollections([]CollectionField{}).
				SetOnlineStatusVisible(true).
				SetInputMode("commands")
			if err := b.Run(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "400") {
				t.Fatalf("setup failure was not returned: %v", err)
			}
		})
	}
}

func TestNewRejectsMultipleOptions(t *testing.T) {
	b, err := New(context.Background(), "id", "secret", &Opts{}, &Opts{})
	if err == nil || b != nil {
		t.Fatalf("expected multiple options to fail, got bot=%v, error=%v", b, err)
	}
}

func TestSetOnlineStatusVisible(t *testing.T) {
	var visible bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/bot/bot" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var body map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || body["onlineStatusVisible"] != visible {
			t.Errorf("unexpected visibility body: %v, %v", body, err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	bot := &Bot{ID: "bot", host: server.URL, tk: &token{AccessToken: "token"}, httpClient: server.Client()}
	for _, visible = range []bool{true, false} {
		if err := bot.SetOnlineStatusVisible(context.Background(), visible); err != nil {
			t.Fatal(err)
		}
	}

	server.Close()
	if err := bot.SetOnlineStatusVisible(context.Background(), true); err == nil {
		t.Fatal("request failure was not returned")
	}
}

func newTestBot() *Bot {
	return &Bot{
		Messages: make(chan *Message, 1),
		Commands: make(chan *Message, 1),
		commands: make(map[string]*Command),
	}
}
