package botcomm

import (
	"testing"
	"time"
)

func TestGetSubFromToken(t *testing.T) {
	// header {"alg":"none"} . payload {"sub":"bot-1","scope":"bot"} . signature
	tk := "eyJhbGciOiJub25lIn0.eyJzdWIiOiJib3QtMSIsInNjb3BlIjoiYm90In0.sig"
	sub, err := getSubFromToken(tk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub != "bot-1" {
		t.Fatalf("expected sub bot-1, got %q", sub)
	}

	if _, err := getSubFromToken("not-a-jwt"); err == nil {
		t.Fatal("expected an error for a malformed token")
	}
}

func TestRegrantInterval(t *testing.T) {
	if got := regrantInterval(3600); got != 59*time.Minute {
		t.Fatalf("expected 59m for a 1h token, got %v", got)
	}

	if got := regrantInterval(30); got != regrantRetry {
		t.Fatalf("expected the %v floor for a short-lived token, got %v", regrantRetry, got)
	}
}
