package botcomm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Attachment is an uploaded image or file embedded in a message.
type Attachment struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Mime         string `json:"mime"`
	Size         int64  `json:"size"`
	Name         string `json:"name,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Thumb        bool   `json:"thumb,omitempty"`
	URL          string `json:"url,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// UploadAttachment uploads bytes for a session. Include the returned ID in
// SendOptions.Attachments within ten minutes to attach it to a message.
func (b *Bot) UploadAttachment(ctx context.Context, sessionID, name string, data []byte) (*Attachment, error) {
	attachment := &Attachment{}
	path := fmt.Sprintf("/session/%s/attachment?name=%s", url.PathEscape(sessionID), url.QueryEscape(name))
	if err := b.requestContent(ctx, http.MethodPost, path, data, "application/octet-stream", "", attachment); err != nil {
		return nil, fmt.Errorf("failed to upload attachment: %w", err)
	}

	return attachment, nil
}
