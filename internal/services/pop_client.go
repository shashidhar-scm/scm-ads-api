package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PopAPI interface {
	CampaignImpressions(ctx context.Context, campaignID string) (*CampaignImpressions, error)
}

type PopClient struct {
	baseURL    string
	httpClient *http.Client
}

type PosterImpression struct {
	PosterID    string `json:"poster_id"`
	PosterName  string `json:"poster_name"`
	Impressions int64  `json:"impressions"`
	PlayTime    int64  `json:"play_time"`
}

type CampaignImpressions struct {
	CampaignID  string             `json:"campaign_id"`
	Impressions int64              `json:"impressions"`
	Posters     []PosterImpression `json:"posters"`
}

func NewPopClient(baseURL string) *PopClient {
	return &PopClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *PopClient) SetHTTPClient(hc *http.Client) {
	if hc != nil {
		c.httpClient = hc
	}
}

func (c *PopClient) CampaignImpressions(ctx context.Context, campaignID string) (*CampaignImpressions, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, fmt.Errorf("pop baseURL is required")
	}
	if strings.TrimSpace(campaignID) == "" {
		return nil, fmt.Errorf("campaignID is required")
	}

	u, err := url.Parse(c.baseURL + "/pop/impressions")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("campaign_id", campaignID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pop impressions request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out CampaignImpressions
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("pop impressions: invalid json: %w", err)
	}
	return &out, nil
}
