package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+pathHealth, nil)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
