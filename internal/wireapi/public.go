package wireapi

import "time"

// ChallengeListItem is one row in GET /api/challenges.
type ChallengeListItem struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description,omitempty"`
	BundleSize  int64     `json:"bundle_size"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateSuiteRequest is the body of POST /api/suites.
type CreateSuiteRequest struct {
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	ChallengeID    int            `json:"challenge_id"`
	Parameters     map[string]any `json:"parameters,omitempty"`
	Seeds          []string       `json:"seeds"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
	Scoring        map[string]any `json:"scoring,omitempty"`
}

// SuiteDetail is the response shape for GET /api/suites/{id} and a single
// element of GET /api/suites.
type SuiteDetail struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Challenge      ChallengeListItem `json:"challenge"`
	Parameters     map[string]any    `json:"parameters,omitempty"`
	Seeds          []string          `json:"seeds"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Scoring        map[string]any    `json:"scoring,omitempty"`
	Sealed         bool              `json:"sealed"`
	CreatedAt      time.Time         `json:"created_at"`
}

// SubmissionSummary is one row in GET /api/submissions.
type SubmissionSummary struct {
	ID             int       `json:"id"`
	Name           string    `json:"name,omitempty"`
	Submitter      string    `json:"submitter,omitempty"`
	ArtifactName   string    `json:"artifact_name"`
	ArtifactSize   int64     `json:"artifact_size"`
	ArtifactSha256 string    `json:"artifact_sha256,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	// Runs is the aggregated count breakdown across every run this
	// submission has produced (across all suites). Drives the
	// status pills and action button gating on the /submissions list.
	Runs RunCounts `json:"runs"`
}

// SubmissionDetail is the response for GET /api/submissions/{id}, plus the
// runs that have been spawned for the submission across suites.
type SubmissionDetail struct {
	SubmissionSummary
	Runs []RunSummary `json:"runs"`
}

// RunSummary is the small per-run representation embedded in submission and
// leaderboard responses.
type RunSummary struct {
	ID         int        `json:"id"`
	SuiteID    int        `json:"suite_id"`
	Seed       string     `json:"seed"`
	Status     string     `json:"status"`
	Score      *float64   `json:"score,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// RunDetail is the response for GET /api/runs/{id}, including the opaque
// result blob the wrapper produced and the captured output tails.
type RunDetail struct {
	RunSummary
	SubmissionID   int            `json:"submission_id"`
	SubmissionName string         `json:"submission_name,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	WorkerID       string         `json:"worker_id,omitempty"`
	ClaimedAt      *time.Time     `json:"claimed_at,omitempty"`
	Stdout         string         `json:"stdout,omitempty"`
	Stderr         string         `json:"stderr,omitempty"`
}

// LeaderboardEntry is one row in GET /api/suites/{id}/leaderboard.
// Metrics is nil until at least one run for the submission has succeeded.
type LeaderboardEntry struct {
	SubmissionID int       `json:"submission_id"`
	Name         string    `json:"name,omitempty"`
	Submitter    string    `json:"submitter,omitempty"`
	Metrics      *Metrics  `json:"metrics,omitempty"`
	// CI95HalfWidth is the half-width of the 95% confidence interval around
	// the mean (1.96 × stddev / √n). Mean ± this value brackets the true
	// mean with 95% confidence under the CLT. 0 when n < 2.
	CI95HalfWidth float64 `json:"ci95_half_width"`
	// PValueVsLeader is the two-tailed paired t-test p-value (normal
	// approximation) comparing this submission's per-seed scores to the
	// top-mean submission's. nil for the leader itself.
	PValueVsLeader *float64  `json:"p_value_vs_leader,omitempty"`
	Runs           RunCounts `json:"runs"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

type Metrics struct {
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	Mode   float64 `json:"mode"`
	Max    float64 `json:"max"`
	Min    float64 `json:"min"`
	Stddev float64 `json:"stddev"`
}

// QueueStatus is the response of GET /api/queue. ETA is null when there is
// no recent throughput to extrapolate from (e.g., nothing's run yet).
type QueueStatus struct {
	PendingTotal     int               `json:"pending_total"`
	ActiveWorkers    int               `json:"active_workers"`
	ThroughputPerSec float64           `json:"throughput_per_sec"`
	EtaSeconds       *float64          `json:"eta_seconds,omitempty"`
	BySuite          []QueueSuiteEntry `json:"by_suite"`
}

type QueueSuiteEntry struct {
	SuiteID        int     `json:"suite_id"`
	Name           string  `json:"name"`
	Pending        int     `json:"pending"`
	MeanDurationMs float64 `json:"mean_duration_ms"`
}

type RunCounts struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	TimedOut  int `json:"timed_out"`
	Pending   int `json:"pending"`
	Cancelled int `json:"cancelled"`
}

// SubmissionDistribution is one series in GET /api/suites/{id}/distributions —
// every succeeded run's score for a submission, used to overlay distributions
// in the cross-submission comparison view.
type SubmissionDistribution struct {
	SubmissionID int       `json:"submission_id"`
	Name         string    `json:"name,omitempty"`
	Submitter    string    `json:"submitter,omitempty"`
	Scores       []float64 `json:"scores"`
	SubmittedAt  time.Time `json:"submitted_at"`
}

// ExportRow is one run in GET /api/suites/{id}/export?format=json. The CSV
// form has the same columns in the same order.
type ExportRow struct {
	RunID            int        `json:"run_id"`
	SubmissionID     int        `json:"submission_id"`
	SubmissionName   string     `json:"submission_name,omitempty"`
	SubmissionSha256 string     `json:"submission_sha256,omitempty"`
	Submitter        string     `json:"submitter,omitempty"`
	SuiteID          int        `json:"suite_id"`
	Seed             string     `json:"seed"`
	Status         string     `json:"status"`
	Score          *float64   `json:"score,omitempty"`
	DurationMs     *int64     `json:"duration_ms,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	Error          string     `json:"error,omitempty"`
}
