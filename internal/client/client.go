package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL        string
	token          string
	organisationID string
	httpClient     *http.Client
}

func New(baseURL, token, organisationID string) *Client {
	return &Client{
		baseURL:        baseURL,
		token:          token,
		organisationID: organisationID,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) OrganisationID() string { return c.organisationID }
func (c *Client) BaseURL() string        { return c.baseURL }

type apiResponse struct {
	Code   int             `json:"code"`
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

// APIError carries the parsed error envelope from the Bahriya API so callers
// can branch on HTTP status (e.g. 404 for not-found, 409 for conflict).
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bahriya api error %d %s: %s", e.StatusCode, e.Status, e.Body)
}

// Do sends an authenticated JSON request and returns the parsed `data` field
// from the response envelope plus the HTTP status. Retries idempotent verbs
// (GET, DELETE) on transient 5xx errors. body may be nil.
func (c *Client) Do(ctx context.Context, method, path string, body any) (json.RawMessage, int, error) {
	var encoded []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		encoded = b
	}
	url := c.baseURL + path

	attempts := 1
	if method == http.MethodGet || method == http.MethodDelete {
		attempts = 3
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		var reader io.Reader
		if encoded != nil {
			reader = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return nil, 0, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if i < attempts-1 {
				time.Sleep(backoff(i))
				continue
			}
			return nil, 0, fmt.Errorf("http request failed: %w", err)
		}
		respBody, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode >= 500 && i < attempts-1 {
			lastErr = fmt.Errorf("server %d: %s", res.StatusCode, string(respBody))
			time.Sleep(backoff(i))
			continue
		}

		var parsed apiResponse
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &parsed); err != nil {
				return nil, res.StatusCode, fmt.Errorf("decode response (status %d): %w; body=%s", res.StatusCode, err, string(respBody))
			}
		}
		if res.StatusCode >= 400 {
			return parsed.Data, res.StatusCode, &APIError{
				StatusCode: res.StatusCode,
				Status:     parsed.Status,
				Body:       string(parsed.Data),
			}
		}
		return parsed.Data, res.StatusCode, nil
	}
	return nil, 0, errors.Join(errors.New("request failed after retries"), lastErr)
}

// HandleExists checks the dedicated /{resource}/{handle} endpoint to see if
// a handle is already in use in the configured organisation. Returns true
// for 200, false for 404, error otherwise.
func (c *Client) HandleExists(ctx context.Context, resource, handle string) (bool, error) {
	path := fmt.Sprintf("/organisations/%s/%s/%s", c.organisationID, resource, handle)
	_, status, err := c.Do(ctx, http.MethodGet, path, nil)
	if status == http.StatusOK {
		return true, nil
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

func backoff(attempt int) time.Duration {
	d := time.Duration(1<<attempt) * time.Second
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}
