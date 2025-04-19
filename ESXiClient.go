package main

import (
	"context"
	"io"
	"net/http"
)

// ESXiClient represents a client for interacting with the ESXi REST API
type ESXiClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// ESXi API client methods can be expanded as needed
func (c *ESXiClient) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Accept", "application/json")

	return c.HTTPClient.Do(req)
}

func (c *ESXiClient) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.HTTPClient.Do(req)
}
