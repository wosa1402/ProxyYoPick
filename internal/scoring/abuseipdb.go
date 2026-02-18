package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type abuseIPDBClient struct {
	apiKey string
}

func newAbuseIPDBClient(apiKey string) *abuseIPDBClient {
	return &abuseIPDBClient{apiKey: apiKey}
}

func (c *abuseIPDBClient) Name() string { return "abuseipdb" }

type abuseIPDBResponse struct {
	Data struct {
		AbuseConfidenceScore int `json:"abuseConfidenceScore"`
	} `json:"data"`
}

func (c *abuseIPDBClient) Score(ctx context.Context, ip string) (int, error) {
	url := fmt.Sprintf("https://api.abuseipdb.com/api/v2/check?ipAddress=%s&maxAgeInDays=90", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("abuseipdb request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("abuseipdb unexpected status: %d", resp.StatusCode)
	}

	var result abuseIPDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("abuseipdb decode: %w", err)
	}

	return result.Data.AbuseConfidenceScore, nil
}
