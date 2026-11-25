package botcomm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// CollectionField declares a session-scoped list in native chat settings.
type CollectionField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	MaxItems    int    `json:"maxItems"`
}

type CollectionItem struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// CollectionRequest arrives outside chat history. For add/remove, atomically
// compare Revision with stored state, mutate, and increment the revision in the
// same write. Reject mismatches without changing state, including duplicate
// requests after a lost response. Never execute an expired request.
type CollectionRequest struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionID"`
	Key       string    `json:"key"`
	Action    string    `json:"action"`
	Value     string    `json:"value,omitempty"`
	ItemID    string    `json:"itemID,omitempty"`
	Revision  *int64    `json:"revision,omitempty"`
	ExpiresTS time.Time `json:"expiresTS"`
}

type CollectionResult struct {
	Items    []CollectionItem `json:"items"`
	Revision int64            `json:"revision"`
	Error    string           `json:"error,omitempty"`
	Changed  bool             `json:"changed,omitempty"`
}

func (b *Bot) RegisterCollections(ctx context.Context, fields []CollectionField) error {
	if fields == nil {
		fields = []CollectionField{}
	}

	return b.do(ctx, http.MethodPost, fmt.Sprintf("/bot/%s", b.ID), map[string]any{"collections": fields}, nil)
}

func (b *Bot) RespondCollection(ctx context.Context, req *CollectionRequest, result CollectionResult) error {
	path := fmt.Sprintf(
		"/session/%s/collections/%s/result/%s",
		url.PathEscape(req.SessionID), url.PathEscape(req.Key), url.PathEscape(req.ID),
	)

	return b.do(ctx, http.MethodPost, path, result, nil)
}
