package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PlaceExchangeClient exchanges username/password credentials for a Place Exchange access token.
type PlaceExchangeClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewPlaceExchangeClient constructs a client with sane defaults.
func NewPlaceExchangeClient(baseURL, username, password string) *PlaceExchangeClient {
	return &PlaceExchangeClient{
		baseURL:    strings.TrimSpace(baseURL),
		username:   strings.TrimSpace(username),
		password:   strings.TrimSpace(password),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// SetHTTPClient allows overriding the default HTTP client (useful for tests).
func (c *PlaceExchangeClient) SetHTTPClient(hc *http.Client) {
	if hc != nil {
		c.httpClient = hc
	}
}

// FetchToken performs the credential exchange and returns the access token string.
func (c *PlaceExchangeClient) FetchToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return "", errors.New("place exchange base URL is required")
	}
	if strings.TrimSpace(c.username) == "" || strings.TrimSpace(c.password) == "" {
		return "", errors.New("place exchange credentials are required")
	}

	payload := map[string]string{
		"username": c.username,
		"password": c.password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("place exchange login failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("place exchange login: invalid json: %w", err)
	}

	for _, key := range []string{"access_token", "token", "accessToken"} {
		if v, ok := parsed[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s, nil
			}
		}
	}

	return "", errors.New("place exchange login response missing access token")
}
