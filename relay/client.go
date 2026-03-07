package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) SetRoute(playerIP, backend string) error {
	body, _ := json.Marshal(map[string]string{
		"player_ip": playerIP,
		"backend":   backend,
	})

	resp, err := c.http.Post(c.baseURL+"/routes", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("peel request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("peel returned %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) DeleteRoute(playerIP string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/routes/"+playerIP, nil)
	if err != nil {
		return fmt.Errorf("peel request failed: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("peel request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("peel returned %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) ListRoutes() (map[string]string, error) {
	resp, err := c.http.Get(c.baseURL + "/routes")
	if err != nil {
		return nil, fmt.Errorf("peel request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("peel returned %d", resp.StatusCode)
	}

	var routes map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("failed to decode routes: %w", err)
	}

	return routes, nil
}
