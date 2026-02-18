package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type scamalyticsClient struct {
	username string
	apiKey   string
}

func newScamalyticsClient(username, apiKey string) *scamalyticsClient {
	return &scamalyticsClient{username: username, apiKey: apiKey}
}

func (c *scamalyticsClient) Name() string { return "scamalytics" }

type scamalyticsResponse struct {
	Score  int    `json:"score"`
	Status string `json:"status"`
}

func (c *scamalyticsClient) Score(ctx context.Context, ip string) (int, error) {
	url := fmt.Sprintf("https://api11.scamalytics.com/v3/%s?key=%s&ip=%s",
		c.username, c.apiKey, ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("scamalytics request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("scamalytics unexpected status: %d", resp.StatusCode)
	}

	var result scamalyticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("scamalytics decode: %w", err)
	}

	if result.Status == "error" {
		return 0, fmt.Errorf("scamalytics error response")
	}

	return result.Score, nil
}
