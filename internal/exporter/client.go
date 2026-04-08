package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"billing-service/internal/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) FetchLatestSnapshot(ctx context.Context) (model.Snapshot, error) {
	endpoint, err := url.JoinPath(c.baseURL, "/v1/snapshots/latest")
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("build snapshot endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("build snapshot request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("fetch snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.Snapshot{}, fmt.Errorf("fetch snapshot: unexpected status %s", resp.Status)
	}

	var snapshot model.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return model.Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	return snapshot, nil
}
