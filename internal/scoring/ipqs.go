package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ipqsClient struct {
	apiKey string
}

func newIPQSClient(apiKey string) *ipqsClient {
	return &ipqsClient{apiKey: apiKey}
}

func (c *ipqsClient) Name() string { return "ipqs" }

type ipqsResponse struct {
	Success    bool   `json:"success"`
	FraudScore int    `json:"fraud_score"`
	Message    string `json:"message"`
}

func (c *ipqsClient) Score(ctx context.Context, ip string) (int, error) {
	url := fmt.Sprintf("https://ipqualityscore.com/api/json/ip/%s/%s?strictness=0", c.apiKey, ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("ipqs request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ipqs unexpected status: %d", resp.StatusCode)
	}

	var result ipqsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("ipqs decode: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("ipqs error: %s", result.Message)
	}

	return result.FraudScore, nil
}
