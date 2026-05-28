// Package wireapi holds the JSON wire types shared between the server's
// internal worker API and the worker HTTP client.
package wireapi

import "time"

// ClaimResponse is the body of POST /internal/runs/claim. The server returns
// 204 No Content when there is no pending work; otherwise this payload.
type ClaimResponse struct {
	ID         int        `json:"id"`
	Seed       string     `json:"seed"`
	Suite      Suite      `json:"suite"`
	Challenge  Challenge  `json:"challenge"`
	Submission Submission `json:"submission"`
}

type Suite struct {
	ID             int            `json:"id"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	Parameters     map[string]any `json:"parameters,omitempty"`
}

type Challenge struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	Manifest map[string]any `json:"manifest"`
}

type Submission struct {
	ID           int    `json:"id"`
	ArtifactName string `json:"artifact_name"`
}

// ResultRequest is the body of POST /internal/runs/{id}/result.
type ResultRequest struct {
	Status    string         `json:"status"` // succeeded | failed | timed_out
	Score     *float64       `json:"score,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	Stdout    string         `json:"stdout,omitempty"`
	Stderr    string         `json:"stderr,omitempty"`
}
