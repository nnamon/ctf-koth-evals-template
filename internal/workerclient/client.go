// Package workerclient is the thin HTTP client workers use to talk to the
// server's internal API. It hides JSON encoding, auth headers, and the
// 204-means-no-work convention behind a few typed methods.
package workerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nnamon/ctf-koth-evals-template/internal/wireapi"
)

type Client struct {
	baseURL  string
	token    string
	workerID string
	http     *http.Client
}

type Options struct {
	BaseURL  string
	Token    string
	WorkerID string
	Timeout  time.Duration
}

func New(opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:  strings.TrimRight(opts.BaseURL, "/"),
		token:    opts.Token,
		workerID: opts.WorkerID,
		http:     &http.Client{Timeout: timeout},
	}
}

// Claim asks the server for one pending run. Returns (nil, nil) when the
// server has no work (HTTP 204).
func (c *Client) Claim(ctx context.Context) (*wireapi.ClaimResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/internal/runs/claim", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, nil
	case http.StatusOK:
		var out wireapi.ClaimResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, fmt.Errorf("decode claim: %w", err)
		}
		return &out, nil
	default:
		return nil, statusError("claim", resp)
	}
}

// FetchBundle streams the challenge bundle bytes for the given content hash.
func (c *Client) FetchBundle(ctx context.Context, hash string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/internal/bundles/"+hash, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError("fetch bundle", resp)
	}
	return io.ReadAll(resp.Body)
}

// FetchSubmission streams the submitted artifact bytes for the given ID.
func (c *Client) FetchSubmission(ctx context.Context, id int) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/internal/submissions/%d/artifact", id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, statusError("fetch submission", resp)
	}
	return io.ReadAll(resp.Body)
}

// RunCancelled reports whether the given run has been cancelled server-side.
// Polled by a worker while a wrapper executes so long-running work can be
// killed promptly. A transient error returns (false, err) — the caller treats
// that as "not cancelled" and retries on the next tick.
func (c *Client) RunCancelled(ctx context.Context, id int) (bool, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/internal/runs/%d/status", id), nil)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, statusError("run status", resp)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("decode status: %w", err)
	}
	return out.Status == "cancelled", nil
}

// ReportResult posts the terminal outcome for a run.
func (c *Client) ReportResult(ctx context.Context, runID int, body wireapi.ResultRequest) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodPost, fmt.Sprintf("/internal/runs/%d/result", runID), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return statusError("report result", resp)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.workerID != "" {
		req.Header.Set("X-Worker-Id", c.workerID)
	}
	return req, nil
}

func statusError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("%s: %s: %s", op, resp.Status, strings.TrimSpace(string(body)))
}
