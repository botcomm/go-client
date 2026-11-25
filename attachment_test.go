package botcomm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadAndSendAttachments(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0x00, 0x10}
	uploads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" || r.Method != http.MethodPost {
			t.Errorf("unexpected upload authentication or method")
		}

		switch r.URL.Path {
		case "/session/s1/attachment":
			uploads++
			body, _ := io.ReadAll(r.Body)
			if string(body) != string(data) || r.Header.Get("Content-Type") != "application/octet-stream" || r.URL.Query().Get("name") != "photo & caption.jpg" {
				t.Errorf("upload lost bytes, content type or filename: %s", r.URL)
			}

			if uploads == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)

				return
			}

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(Attachment{ID: "a1", Kind: "image", Width: 640, Height: 480})
		case "/session/s1/message":
			var body struct {
				Content     string   `json:"content"`
				Attachments []string `json:"attachments"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}

			if r.Header.Get("Content-Type") != "application/json" || body.Content != "Caption" || len(body.Attachments) != 1 || body.Attachments[0] != "a1" {
				t.Errorf("message lost album: %+v", body)
			}

			_ = json.NewEncoder(w).Encode(Message{ID: "m1", Content: body.Content, Attachments: []Attachment{{ID: "a1", Kind: "image"}}})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	b := &Bot{host: srv.URL, httpClient: srv.Client(), tk: &token{AccessToken: "test"}}
	a, err := b.UploadAttachment(context.Background(), "s1", "photo & caption.jpg", data)
	if err != nil || a.ID != "a1" || a.Width != 640 || uploads != 2 {
		t.Fatalf("upload=%+v, attempts=%d, err=%v", a, uploads, err)
	}

	msg, err := b.SendWithOptions(context.Background(), "s1", "Caption", SendOptions{Attachments: []string{a.ID}})
	if err != nil || len(msg.Attachments) != 1 || msg.Attachments[0].ID != a.ID {
		t.Fatalf("message=%+v, err=%v", msg, err)
	}
}
