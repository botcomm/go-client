package botcomm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// do performs an authenticated JSON request against the API, decoding the
// response into out when it is not nil.
func (b *Bot) do(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	if body != nil {
		var err error
		if data, err = json.Marshal(body); err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	key := ""
	if input, ok := body.(MessageInput); ok {
		key = input.IdempotencyKey
	}

	return b.requestContent(ctx, method, path, data, "application/json", key, out)
}

func (b *Bot) requestContent(ctx context.Context, method, path string, data []byte, contentType, idempotencyKey string, out any) error {
	resp, err := b.send(ctx, method, path, data, contentType, idempotencyKey)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		_ = resp.Body.Close()
		wait := time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			wait = min(time.Duration(s)*time.Second, 30*time.Second)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		if resp, err = b.send(ctx, method, path, data, contentType, idempotencyKey); err != nil {
			return err
		}
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: %d - %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (b *Bot) send(ctx context.Context, method, path string, data []byte, contentType, idempotencyKey string) (*http.Response, error) {
	var reader io.Reader
	if data != nil {
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, b.host+path, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", contentType)
	}

	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.accessToken())

	resp, err := b.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (b *Bot) client() *http.Client {
	if b.httpClient != nil {
		return b.httpClient
	}

	return http.DefaultClient
}
