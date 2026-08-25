package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// stackryzeClient talks to the Stackryze control plane with a Bearer token.
type stackryzeClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(baseURL, token string) *stackryzeClient {
	if baseURL == "" {
		baseURL = "https://api-dns.stackryze.com/api"
	}
	return &stackryzeClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *stackryzeClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s %s: %s (%s)", method, path, res.Status, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// discoverZones lists the account's zone names.
func (c *stackryzeClient) discoverZones(ctx context.Context) ([]string, error) {
	var resp struct {
		Zones []struct {
			Name string `json:"name"`
		} `json:"zones"`
	}
	if err := c.do(ctx, "GET", "/zones", nil, &resp); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(resp.Zones))
	for _, z := range resp.Zones {
		names = append(names, strings.TrimSuffix(z.Name, "."))
	}
	return names, nil
}

// metricPayload is the aggregated snapshot pushed per zone per window.
type metricPayload struct {
	ZoneName     string  `json:"zoneName"`
	Region       string  `json:"region"`
	AgentID      string  `json:"agentId"`
	WindowStart  string  `json:"windowStart"`
	WindowEnd    string  `json:"windowEnd"`
	Checks       int     `json:"checks"`
	SuccessRate  float64 `json:"successRate"`
	LatencyP50   float64 `json:"latencyP50"`
	LatencyP95   float64 `json:"latencyP95"`
	TargetsUp    int     `json:"targetsUp"`
	TargetsDown  int     `json:"targetsDown"`
}

func (c *stackryzeClient) pushMetrics(ctx context.Context, p metricPayload) error {
	return c.do(ctx, "POST", "/edge/metrics", p, nil)
}
