package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type CityPostDevice struct {
	DeviceName string `json:"device_name"`
	HostName   string `json:"host_name"`
}

type CityPostConsoleClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	authScheme string
	tokenTTL   time.Duration

	mu        sync.Mutex
	token     string
	tokenExp  time.Time
}

func NewCityPostConsoleClient(baseURL, username, password string) *CityPostConsoleClient {
	return &CityPostConsoleClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		authScheme: "Bearer",
		tokenTTL:   30 * time.Minute,
	}
}

func (c *CityPostConsoleClient) SetHTTPClient(hc *http.Client) {
	if hc != nil {
		c.httpClient = hc
	}
}

func (c *CityPostConsoleClient) SetAuthScheme(scheme string) {
	if strings.TrimSpace(scheme) != "" {
		c.authScheme = scheme
	}
}

func (c *CityPostConsoleClient) SetTokenTTL(ttl time.Duration) {
	if ttl > 0 {
		c.tokenTTL = ttl
	}
}

func (c *CityPostConsoleClient) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		t := c.token
		c.mu.Unlock()
		return t, nil
	}
	c.mu.Unlock()

	tok, err := c.login(ctx)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.token = tok
	c.tokenExp = time.Now().Add(c.tokenTTL)
	c.mu.Unlock()

	return tok, nil
}

func (c *CityPostConsoleClient) login(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return "", errors.New("citypost baseURL is required")
	}
	if strings.TrimSpace(c.username) == "" || strings.TrimSpace(c.password) == "" {
		return "", errors.New("citypost username/password are required")
	}

	loginURL := c.baseURL + "/login/"
	payload := map[string]string{
		"username": c.username,
		"password": c.password,
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader(b))
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

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("citypost login failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("citypost login: invalid json: %w", err)
	}

	// Try common token field names.
	for _, k := range []string{"token", "access", "access_token", "jwt"} {
		if v, ok := out[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s, nil
			}
		}
	}

	return "", errors.New("citypost login response did not include token")
}

func (c *CityPostConsoleClient) ListDevices(ctx context.Context, project, region string) ([]CityPostDevice, error) {
	project = strings.TrimSpace(project)
	region = strings.TrimSpace(region)
	if project == "" || region == "" {
		return nil, errors.New("project and region are required")
	}

	token, err := c.ensureToken(ctx)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(c.baseURL + "/device/")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("project", project)
	q.Set("region", region)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", strings.TrimSpace(c.authScheme)+" "+strings.TrimSpace(token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("citypost list devices failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		var wrapper map[string]any
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil, fmt.Errorf("citypost list devices: invalid json: %w", err)
		}

		tryKeys := []string{"results", "data", "devices", "items"}
		var rawList []any
		for _, k := range tryKeys {
			if v, ok := wrapper[k]; ok {
				if raw, ok := v.([]any); ok {
					rawList = raw
					break
				}
			}
		}
		if rawList == nil {
			if len(wrapper) == 1 {
				for _, v := range wrapper {
					if raw, ok := v.([]any); ok {
						rawList = raw
						break
					}
				}
			}
		}
		if rawList == nil {
			return nil, fmt.Errorf("citypost list devices: unexpected json object")
		}

		arr = make([]map[string]any, 0, len(rawList))
		for _, item := range rawList {
			if m, ok := item.(map[string]any); ok {
				arr = append(arr, m)
			}
		}
	}

	out := make([]CityPostDevice, 0, len(arr))
	for _, m := range arr {
		dev := CityPostDevice{}
		if v, ok := m["name"]; ok {
			if s, ok := v.(string); ok {
				dev.DeviceName = s
			}
		}
		if v, ok := m["host_name"]; ok {
			if s, ok := v.(string); ok {
				dev.HostName = s
			}
		}
		if dev.DeviceName == "" && dev.HostName == "" {
			continue
		}
		out = append(out, dev)
	}

	return out, nil
}
