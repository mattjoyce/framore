package ductile

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	URL    string
	Secret string
	HTTP   *http.Client
}

func NewClient(url, secret string) *Client {
	return &Client{
		URL:    url,
		Secret: secret,
		HTTP:   &http.Client{},
	}
}

// Sign computes HMAC-SHA256 and returns "sha256=<hex>"
func (c *Client) Sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(c.Secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Post sends a signed JSON payload to the Ductile webhook
func (c *Client) Post(ctx context.Context, payload any) ([]byte, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ductile-Signature-256", c.Sign(bodyBytes))

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ductile returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
