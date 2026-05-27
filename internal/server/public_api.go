package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/challenge"
	"github.com/nnamon/ctf-koth-evals-template/ent/run"
	"github.com/nnamon/ctf-koth-evals-template/ent/submission"
	"github.com/nnamon/ctf-koth-evals-template/ent/suite"
	"github.com/nnamon/ctf-koth-evals-template/ent/worker"
	"github.com/nnamon/ctf-koth-evals-template/internal/scoring"
	"github.com/nnamon/ctf-koth-evals-template/internal/wireapi"
)

func (s *Deps) mountPublic(api chi.Router) {
	api.Get("/me", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"authenticated":true}`))
	})

	api.Get("/challenges", s.listChallenges)

	api.Get("/suites", s.listSuites)
	api.Post("/suites", s.createSuite)
	api.Get("/suites/{id}", s.getSuite)
	api.Get("/suites/{id}/leaderboard", s.getLeaderboard)

	api.Get("/submissions", s.listSubmissions)
	api.Post("/submissions", s.createSubmission)
	api.Get("/submissions/{id}", s.getSubmission)

	api.Get("/runs/{id}", s.getRun)

	api.Get("/queue", s.getQueue)
}

// ----- challenges -----

func (s *Deps) listChallenges(w http.ResponseWriter, req *http.Request) {
	rows, err := s.Client.Challenge.Query().
		Order(ent.Desc(challenge.FieldCreatedAt)).
		Select(
			challenge.FieldID,
			challenge.FieldName,
			challenge.FieldVersion,
			challenge.FieldDescription,
			challenge.FieldBundleSize,
			challenge.FieldCreatedAt,
		).
		All(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	out := make([]wireapi.ChallengeListItem, 0, len(rows))
	for _, c := range rows {
		out = append(out, challengeListItem(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func challengeListItem(c *ent.Challenge) wireapi.ChallengeListItem {
	return wireapi.ChallengeListItem{
		ID:          c.ID,
		Name:        c.Name,
		Version:     c.Version,
		Description: c.Description,
		BundleSize:  c.BundleSize,
		CreatedAt:   c.CreatedAt,
	}
}

// ----- suites -----

func (s *Deps) listSuites(w http.ResponseWriter, req *http.Request) {
	rows, err := s.Client.Suite.Query().
		WithChallenge().
		Order(ent.Desc(suite.FieldCreatedAt)).
		All(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	out := make([]wireapi.SuiteDetail, 0, len(rows))
	for _, r := range rows {
		out = append(out, suiteDetail(r))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Deps) createSuite(w http.ResponseWriter, req *http.Request) {
	var body wireapi.CreateSuiteRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if body.ChallengeID == 0 {
		http.Error(w, "challenge_id is required", http.StatusBadRequest)
		return
	}
	if len(body.Seeds) == 0 {
		http.Error(w, "seeds must be non-empty", http.StatusBadRequest)
		return
	}
	if body.TimeoutSeconds == 0 {
		body.TimeoutSeconds = 60
	}

	ch, err := s.Client.Challenge.Get(req.Context(), body.ChallengeID)
	if err != nil {
		if ent.IsNotFound(err) {
			http.Error(w, "challenge not found", http.StatusBadRequest)
			return
		}
		httpError(w, http.StatusInternalServerError, "lookup challenge: %v", err)
		return
	}

	create := s.Client.Suite.Create().
		SetName(body.Name).
		SetDescription(body.Description).
		SetChallenge(ch).
		SetSeeds(body.Seeds).
		SetTimeoutSeconds(body.TimeoutSeconds)
	if body.Parameters != nil {
		create = create.SetParameters(body.Parameters)
	}
	if body.Scoring != nil {
		create = create.SetScoring(body.Scoring)
	}
	created, err := create.Save(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "create: %v", err)
		return
	}

	created.Edges.Challenge = ch
	writeJSON(w, http.StatusCreated, suiteDetail(created))
}

func (s *Deps) getSuite(w http.ResponseWriter, req *http.Request) {
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	r, err := s.Client.Suite.Query().
		Where(suite.IDEQ(id)).
		WithChallenge().
		Only(req.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, suiteDetail(r))
}

func suiteDetail(r *ent.Suite) wireapi.SuiteDetail {
	d := wireapi.SuiteDetail{
		ID:             r.ID,
		Name:           r.Name,
		Description:    r.Description,
		Parameters:     r.Parameters,
		Seeds:          r.Seeds,
		TimeoutSeconds: r.TimeoutSeconds,
		Scoring:        r.Scoring,
		Sealed:         r.Sealed,
		CreatedAt:      r.CreatedAt,
	}
	if r.Edges.Challenge != nil {
		d.Challenge = challengeListItem(r.Edges.Challenge)
	}
	return d
}

// ----- submissions -----

func (s *Deps) listSubmissions(w http.ResponseWriter, req *http.Request) {
	rows, err := s.Client.Submission.Query().
		Order(ent.Desc(submission.FieldCreatedAt)).
		Select(
			submission.FieldID,
			submission.FieldName,
			submission.FieldSubmitter,
			submission.FieldArtifactName,
			submission.FieldArtifactSize,
			submission.FieldCreatedAt,
		).
		All(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	out := make([]wireapi.SubmissionSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, submissionSummary(r))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Deps) createSubmission(w http.ResponseWriter, req *http.Request) {
	maxBytes := int64(s.Cfg.MaxUploadMB) * 1024 * 1024
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes+1024)

	if err := req.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}

	suiteIDs, err := parseSuiteIDs(req.MultipartForm.Value["suite_ids"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	submitter := req.FormValue("submitter")
	name := req.FormValue("name")

	file, header, err := req.FormFile("artifact")
	if err != nil {
		http.Error(w, "artifact field is required (multipart file)", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > maxBytes {
		http.Error(w, "artifact exceeds max upload size", http.StatusRequestEntityTooLarge)
		return
	}

	artifact, err := io.ReadAll(file)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "read upload: %v", err)
		return
	}

	suites, err := s.Client.Suite.Query().
		Where(suite.IDIn(suiteIDs...)).
		All(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "lookup suites: %v", err)
		return
	}
	if len(suites) != len(suiteIDs) {
		http.Error(w, "one or more suite_ids do not exist", http.StatusBadRequest)
		return
	}

	if name == "" {
		name = defaultName(header.Filename)
	}

	created, runs, err := s.persistSubmission(req.Context(), suites, name, submitter, header.Filename, artifact)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "persist: %v", err)
		return
	}

	detail := wireapi.SubmissionDetail{
		SubmissionSummary: submissionSummary(created),
		Runs:              make([]wireapi.RunSummary, 0, len(runs)),
	}
	for _, r := range runs {
		detail.Runs = append(detail.Runs, runSummary(r))
	}
	writeJSON(w, http.StatusCreated, detail)
}

func parseSuiteIDs(raw []string) ([]int, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("at least one suite_ids field is required")
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(raw))
	for _, s := range raw {
		// Tolerate "1,2,3" as well as repeated fields.
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, err := strconv.Atoi(part)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("suite_ids: %q is not a positive integer", part)
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one suite id is required")
	}
	return out, nil
}

// persistSubmission creates one Submission and fans out one pending Run per
// (suite × seed). All suites are sealed. Wrapped in a transaction so partial
// failure doesn't leave a submission with missing runs.
func (s *Deps) persistSubmission(ctx context.Context, suites []*ent.Suite, name, submitter, filename string, artifact []byte) (*ent.Submission, []*ent.Run, error) {
	tx, err := s.Client.Tx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	sub, err := tx.Submission.Create().
		SetName(name).
		SetSubmitter(submitter).
		SetArtifactName(filename).
		SetArtifact(artifact).
		SetArtifactSize(int64(len(artifact))).
		Save(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create submission: %w", err)
	}

	totalRuns := 0
	for _, su := range suites {
		totalRuns += len(su.Seeds)
	}
	runs := make([]*ent.Run, 0, totalRuns)
	for _, su := range suites {
		for _, seed := range su.Seeds {
			r, err := tx.Run.Create().
				SetSeed(seed).
				SetSuite(su).
				SetSubmission(sub).
				Save(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("create run: %w", err)
			}
			r.Edges.Suite = su
			runs = append(runs, r)
		}
		if !su.Sealed {
			if _, err := tx.Suite.UpdateOneID(su.ID).SetSealed(true).Save(ctx); err != nil {
				return nil, nil, fmt.Errorf("seal suite %d: %w", su.ID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	committed = true
	return sub, runs, nil
}

func (s *Deps) getSubmission(w http.ResponseWriter, req *http.Request) {
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	sub, err := s.Client.Submission.Query().
		Where(submission.IDEQ(id)).
		WithRuns(func(q *ent.RunQuery) {
			q.Order(ent.Asc(run.FieldID)).WithSuite()
		}).
		Only(req.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	detail := wireapi.SubmissionDetail{
		SubmissionSummary: submissionSummary(sub),
		Runs:              make([]wireapi.RunSummary, 0, len(sub.Edges.Runs)),
	}
	for _, r := range sub.Edges.Runs {
		detail.Runs = append(detail.Runs, runSummary(r))
	}
	writeJSON(w, http.StatusOK, detail)
}

func submissionSummary(r *ent.Submission) wireapi.SubmissionSummary {
	return wireapi.SubmissionSummary{
		ID:           r.ID,
		Name:         r.Name,
		Submitter:    r.Submitter,
		ArtifactName: r.ArtifactName,
		ArtifactSize: r.ArtifactSize,
		CreatedAt:    r.CreatedAt,
	}
}

// defaultName returns the filename without its extension. Used when the
// uploader doesn't supply an explicit name on the submission.
func defaultName(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

// ----- runs -----

func (s *Deps) getRun(w http.ResponseWriter, req *http.Request) {
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	r, err := s.Client.Run.Query().
		Where(run.IDEQ(id)).
		WithSubmission().
		WithSuite().
		Only(req.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	d := wireapi.RunDetail{
		RunSummary: runSummary(r),
		Result:     r.Result,
		WorkerID:   r.WorkerID,
		ClaimedAt:  r.ClaimedAt,
	}
	if r.Edges.Submission != nil {
		d.SubmissionID = r.Edges.Submission.ID
	}
	writeJSON(w, http.StatusOK, d)
}

func runSummary(r *ent.Run) wireapi.RunSummary {
	return wireapi.RunSummary{
		ID:         r.ID,
		SuiteID:    suiteIDFromRun(r),
		Seed:       r.Seed,
		Status:     string(r.Status),
		Score:      r.Score,
		Error:      r.Error,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		CreatedAt:  r.CreatedAt,
	}
}

func suiteIDFromRun(r *ent.Run) int {
	if r.Edges.Suite != nil {
		return r.Edges.Suite.ID
	}
	return 0
}

// ----- leaderboard -----

func (s *Deps) getLeaderboard(w http.ResponseWriter, req *http.Request) {
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if _, err := s.Client.Suite.Get(req.Context(), id); err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "load suite: %v", err)
		return
	}

	runs, err := s.Client.Run.Query().
		Where(run.HasSuiteWith(suite.IDEQ(id))).
		WithSubmission(func(q *ent.SubmissionQuery) {
			q.Select(
				submission.FieldID,
				submission.FieldName,
				submission.FieldSubmitter,
				submission.FieldCreatedAt,
			)
		}).
		All(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "load runs: %v", err)
		return
	}

	type bucket struct {
		name         string
		submitter    string
		submittedAt  time.Time
		scores       []float64
		scoresBySeed map[string]float64
		counts       wireapi.RunCounts
	}
	buckets := map[int]*bucket{}
	for _, r := range runs {
		if r.Edges.Submission == nil {
			continue
		}
		sid := r.Edges.Submission.ID
		b, ok := buckets[sid]
		if !ok {
			b = &bucket{
				name:         r.Edges.Submission.Name,
				submitter:    r.Edges.Submission.Submitter,
				submittedAt:  r.Edges.Submission.CreatedAt,
				scoresBySeed: map[string]float64{},
			}
			buckets[sid] = b
		}
		b.counts.Total++
		switch r.Status {
		case run.StatusSucceeded:
			b.counts.Succeeded++
			if r.Score != nil {
				b.scores = append(b.scores, *r.Score)
				b.scoresBySeed[r.Seed] = *r.Score
			}
		case run.StatusFailed:
			b.counts.Failed++
		case run.StatusTimedOut:
			b.counts.TimedOut++
		case run.StatusPending, run.StatusClaimed, run.StatusRunning:
			b.counts.Pending++
		}
	}

	out := make([]wireapi.LeaderboardEntry, 0, len(buckets))
	for sid, b := range buckets {
		entry := wireapi.LeaderboardEntry{
			SubmissionID: sid,
			Name:         b.name,
			Submitter:    b.submitter,
			Runs:         b.counts,
			SubmittedAt:  b.submittedAt,
		}
		if len(b.scores) > 0 {
			sd := scoring.Aggregate(b.scores, scoring.Stddev)
			entry.Metrics = &wireapi.Metrics{
				Mean:   scoring.Aggregate(b.scores, scoring.Mean),
				Median: scoring.Aggregate(b.scores, scoring.Median),
				Mode:   scoring.Aggregate(b.scores, scoring.Mode),
				Max:    scoring.Aggregate(b.scores, scoring.Max),
				Min:    scoring.Aggregate(b.scores, scoring.Min),
				Stddev: sd,
			}
			if len(b.scores) >= 2 && !math.IsNaN(sd) {
				entry.CI95HalfWidth = 1.96 * sd / math.Sqrt(float64(len(b.scores)))
			}
		}
		out = append(out, entry)
	}

	sortLeaderboard(out)

	// Paired t-test (normal approximation) of every ranked entry vs the
	// post-sort leader. Lookup is by submission_id since `out` was
	// reordered; the buckets map is keyed by it directly.
	for i := range out {
		if out[i].Metrics == nil {
			continue
		}
		leaderBkt := buckets[out[i].SubmissionID]
		for j := i + 1; j < len(out); j++ {
			if out[j].Metrics == nil {
				continue
			}
			otherBkt := buckets[out[j].SubmissionID]
			p := pairedPValue(leaderBkt.scoresBySeed, otherBkt.scoresBySeed)
			if !math.IsNaN(p) {
				out[j].PValueVsLeader = &p
			}
		}
		break // only need to do this for the first ranked entry (the leader)
	}

	writeJSON(w, http.StatusOK, out)
}

// pairedPValue computes the two-tailed p-value of a paired t-test on the
// per-seed score differences between two submissions, using the normal
// approximation (valid for the n ≥ 30 typical of these suites). NaN when
// fewer than two seeds overlap.
func pairedPValue(a, b map[string]float64) float64 {
	var diffs []float64
	for seed, sa := range a {
		if sb, ok := b[seed]; ok {
			diffs = append(diffs, sa-sb)
		}
	}
	if len(diffs) < 2 {
		return math.NaN()
	}
	var sum float64
	for _, d := range diffs {
		sum += d
	}
	mean := sum / float64(len(diffs))
	var sq float64
	for _, d := range diffs {
		sq += (d - mean) * (d - mean)
	}
	sd := math.Sqrt(sq / float64(len(diffs)-1))
	if sd == 0 {
		if mean == 0 {
			return 1.0
		}
		return 0.0
	}
	se := sd / math.Sqrt(float64(len(diffs)))
	z := math.Abs(mean / se)
	return math.Erfc(z / math.Sqrt2)
}

// sortLeaderboard ranks by mean desc; nil metrics last; tiebreak by earliest
// submission. The SPA re-sorts client-side when the user clicks a header.
func sortLeaderboard(entries []wireapi.LeaderboardEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && less(entries[j], entries[j-1]); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

func less(a, b wireapi.LeaderboardEntry) bool {
	if a.Metrics == nil && b.Metrics == nil {
		return a.SubmittedAt.Before(b.SubmittedAt)
	}
	if a.Metrics == nil {
		return false
	}
	if b.Metrics == nil {
		return true
	}
	if a.Metrics.Mean != b.Metrics.Mean {
		return a.Metrics.Mean > b.Metrics.Mean
	}
	return a.SubmittedAt.Before(b.SubmittedAt)
}

// ----- queue ETA -----

func (s *Deps) getQueue(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Count pending+claimed+running runs grouped by suite.
	pendingRuns, err := s.Client.Run.Query().
		Where(run.StatusIn(run.StatusPending, run.StatusClaimed, run.StatusRunning)).
		WithSuite().
		All(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "pending runs: %v", err)
		return
	}
	pendingBySuite := map[int]int{}
	for _, r := range pendingRuns {
		if r.Edges.Suite != nil {
			pendingBySuite[r.Edges.Suite.ID]++
		}
	}

	// Per-suite mean duration of succeeded runs (all-time).
	suites, err := s.Client.Suite.Query().All(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "suites: %v", err)
		return
	}

	entries := make([]wireapi.QueueSuiteEntry, 0, len(suites))
	pendingWorkSeconds := 0.0
	for _, su := range suites {
		succeeded, err := s.Client.Run.Query().
			Where(
				run.HasSuiteWith(suite.IDEQ(su.ID)),
				run.StatusEQ(run.StatusSucceeded),
				run.StartedAtNotNil(),
				run.FinishedAtNotNil(),
			).
			Select(run.FieldStartedAt, run.FieldFinishedAt).
			All(ctx)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "succeeded runs: %v", err)
			return
		}
		var sumMs float64
		for _, r := range succeeded {
			sumMs += float64(r.FinishedAt.Sub(*r.StartedAt) / time.Millisecond)
		}
		meanMs := 0.0
		if len(succeeded) > 0 {
			meanMs = sumMs / float64(len(succeeded))
		}
		pending := pendingBySuite[su.ID]
		if pending == 0 && meanMs == 0 {
			continue
		}
		entries = append(entries, wireapi.QueueSuiteEntry{
			SuiteID:        su.ID,
			Name:           su.Name,
			Pending:        pending,
			MeanDurationMs: meanMs,
		})
		pendingWorkSeconds += float64(pending) * meanMs / 1000
	}

	// Recent throughput: aggregate run-seconds completed against the actual
	// elapsed wall time (not the fixed window) so the metric is honest right
	// after start-up and not artificially squished by an empty cold period.
	const window = 30 * time.Second
	cutoff := time.Now().Add(-window)
	recent, err := s.Client.Run.Query().
		Where(
			run.StatusEQ(run.StatusSucceeded),
			run.FinishedAtGT(cutoff),
			run.StartedAtNotNil(),
		).
		Select(run.FieldStartedAt, run.FieldFinishedAt).
		All(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "recent runs: %v", err)
		return
	}
	var workSeconds float64
	var oldestFinish time.Time
	for _, r := range recent {
		workSeconds += r.FinishedAt.Sub(*r.StartedAt).Seconds()
		if oldestFinish.IsZero() || r.FinishedAt.Before(oldestFinish) {
			oldestFinish = *r.FinishedAt
		}
	}
	var throughput float64
	if len(recent) > 0 {
		wall := time.Since(oldestFinish).Seconds()
		if wall < 1 {
			wall = 1
		}
		throughput = workSeconds / wall
	}

	// Active workers (last_seen within heartbeat timeout).
	workerCutoff := time.Now().Add(-s.Cfg.WorkerTimeout)
	activeWorkers, err := s.Client.Worker.Query().
		Where(worker.LastSeenGT(workerCutoff)).
		Count(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "workers: %v", err)
		return
	}

	resp := wireapi.QueueStatus{
		PendingTotal:     len(pendingRuns),
		ActiveWorkers:    activeWorkers,
		ThroughputPerSec: throughput,
		BySuite:          entries,
	}
	if throughput > 0 {
		eta := pendingWorkSeconds / throughput
		resp.EtaSeconds = &eta
	}
	writeJSON(w, http.StatusOK, resp)
}

// ----- helpers -----

func pathID(w http.ResponseWriter, req *http.Request, key string) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(req, key))
	if err != nil {
		http.Error(w, "bad "+key, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// keep imports clean if errors package isn't otherwise used.
var _ = errors.New
