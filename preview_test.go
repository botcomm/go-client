package botcomm

import (
	"encoding/json"
	"testing"
)

func TestMessagePreview(t *testing.T) {
	var message Message
	if err := json.Unmarshal([]byte(`{"linkPreview":{"url":"https://example.com","siteName":"Example","title":"Article","image":{"id":"image","width":800,"height":400}},"previewTS":"2026-10-09T00:00:00Z"}`), &message); err != nil {
		t.Fatal(err)
	}

	if message.LinkPreview == nil || message.LinkPreview.SiteName != "Example" || message.LinkPreview.Image.Width != 800 || message.PreviewTS == nil {
		t.Fatalf("preview did not decode: %+v", message)
	}
}
