package botcomm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errUnauthorized = errors.New("access token rejected")

type token struct {
	AccessToken string `json:"access_token"`
	ExpireIn    int    `json:"expires_in"`
}

// authorize performs the initial client_credentials grant and starts a
// goroutine that re-grants regrantEarliness before every expiry.
func (b *Bot) authorize(ctx context.Context) error {
	tk, err := b.grant(ctx)
	if err != nil {
		return err
	}

	sub, err := getSubFromToken(tk.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to get sub from token: %w", err)
	}

	b.mu.Lock()
	b.tk = tk
	b.ID = sub
	b.mu.Unlock()

	go b.regrantLoop(ctx, regrantInterval(tk.ExpireIn))

	return nil
}

func (b *Bot) regrantLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tk, err := b.grant(ctx)
			if err != nil {
				log.Printf("[ERROR] failed to refresh access token: %v", err)
				ticker.Reset(regrantRetry)
				continue
			}

			b.mu.Lock()
			b.tk = tk
			b.mu.Unlock()
			ticker.Reset(regrantInterval(tk.ExpireIn))
		case <-ctx.Done():
			return
		}
	}
}

func regrantInterval(expiresIn int) time.Duration {
	interval := time.Duration(expiresIn)*time.Second - regrantEarliness
	if interval < regrantRetry {
		interval = regrantRetry
	}

	return interval
}

// grant exchanges the client credentials for an access token. Bots get no
// refresh token: they re-grant before expiry.
func (b *Bot) grant(ctx context.Context) (*token, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", b.ClientID)
	form.Set("client_secret", b.ClientSecret)

	uri := b.authHost + "/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uri, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request to %s: %w", uri, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := b.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	tk := &token{}
	if err := json.NewDecoder(resp.Body).Decode(tk); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tk.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response")
	}

	return tk, nil
}

func (b *Bot) accessToken() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.tk == nil {
		return ""
	}

	return b.tk.AccessToken
}

// getSubFromToken extracts the sub claim from a JWT without verifying it (the
// server is the source of the token).
func getSubFromToken(tokenString string) (string, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("sub claim not found in token")
	}

	return sub, nil
}
