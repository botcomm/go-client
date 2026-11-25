package botcomm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeaturedBots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/bot/featured" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected request: %s %s, auth %q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}

		_, _ = w.Write([]byte(`[{"id":"b2","handle":"news","name":"News"},{"id":"b1","handle":"weather","name":"Weather"}]`))
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, tk: &token{AccessToken: "test-token"}}
	list, err := b.FeaturedBots(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("featured bots: %+v, %v", list, err)
	}

	if list[0].Handle != "news" || list[1].Name != "Weather" {
		t.Fatalf("unexpected featured bots: %+v %+v", list[0], list[1])
	}
}

func TestFeaturedBotsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL}
	if _, err := b.FeaturedBots(context.Background()); err == nil {
		t.Fatal("expected the featured request failure")
	}
}
