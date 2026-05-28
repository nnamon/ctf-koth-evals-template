package server

import (
	"encoding/csv"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/run"
	"github.com/nnamon/ctf-koth-evals-template/ent/submission"
	"github.com/nnamon/ctf-koth-evals-template/ent/suite"
	"github.com/nnamon/ctf-koth-evals-template/internal/scoring"
	"github.com/nnamon/ctf-koth-evals-template/internal/wireapi"
)

// getDistributions returns the raw per-submission score arrays for a suite —
// the data behind the cross-submission comparison view. Only succeeded runs
// with a score are included. Ordered by mean score descending so the SPA's
// default selection lines up with the leaderboard's top entries.
func (s *Deps) getDistributions(w http.ResponseWriter, req *http.Request) {
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
		Where(
			run.HasSuiteWith(suite.IDEQ(id)),
			run.StatusEQ(run.StatusSucceeded),
			run.ScoreNotNil(),
		).
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
		dist *wireapi.SubmissionDistribution
		mean float64
	}
	byID := map[int]*bucket{}
	for _, r := range runs {
		if r.Edges.Submission == nil || r.Score == nil {
			continue
		}
		sid := r.Edges.Submission.ID
		b, ok := byID[sid]
		if !ok {
			b = &bucket{dist: &wireapi.SubmissionDistribution{
				SubmissionID: sid,
				Name:         r.Edges.Submission.Name,
				Submitter:    r.Edges.Submission.Submitter,
				SubmittedAt:  r.Edges.Submission.CreatedAt,
			}}
			byID[sid] = b
		}
		b.dist.Scores = append(b.dist.Scores, *r.Score)
	}

	out := make([]wireapi.SubmissionDistribution, 0, len(byID))
	for _, b := range byID {
		b.mean = scoring.Aggregate(b.dist.Scores, scoring.Mean)
		out = append(out, *b.dist)
	}
	sort.Slice(out, func(i, j int) bool {
		mi := scoring.Aggregate(out[i].Scores, scoring.Mean)
		mj := scoring.Aggregate(out[j].Scores, scoring.Mean)
		if mi != mj {
			return mi > mj
		}
		return out[i].SubmittedAt.Before(out[j].SubmittedAt)
	})

	writeJSON(w, http.StatusOK, out)
}

// exportSuite streams every run for a suite as a downloadable CSV (default) or
// JSON file — a full dump for offline analysis or archival.
func (s *Deps) exportSuite(w http.ResponseWriter, req *http.Request) {
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	su, err := s.Client.Suite.Get(req.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "load suite: %v", err)
		return
	}

	runs, err := s.Client.Run.Query().
		Where(run.HasSuiteWith(suite.IDEQ(id))).
		Order(ent.Asc(run.FieldID)).
		WithSubmission(func(q *ent.SubmissionQuery) {
			q.Select(submission.FieldID, submission.FieldName, submission.FieldSubmitter)
		}).
		All(req.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, "load runs: %v", err)
		return
	}

	format := req.URL.Query().Get("format")
	base := "suite-" + strconv.Itoa(id) + "-" + slug(su.Name)

	if format == "json" {
		rows := make([]wireapi.ExportRow, 0, len(runs))
		for _, r := range runs {
			rows = append(rows, exportRow(r, id))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.json"`)
		writeJSON(w, http.StatusOK, rows)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"run_id", "submission_id", "submission_name", "submitter", "suite_id",
		"seed", "status", "score", "duration_ms", "started_at", "finished_at",
		"created_at", "error",
	})
	for _, r := range runs {
		row := exportRow(r, id)
		_ = cw.Write([]string{
			strconv.Itoa(row.RunID),
			strconv.Itoa(row.SubmissionID),
			row.SubmissionName,
			row.Submitter,
			strconv.Itoa(row.SuiteID),
			row.Seed,
			row.Status,
			floatOrEmpty(row.Score),
			intOrEmpty(row.DurationMs),
			timeOrEmpty(row.StartedAt),
			timeOrEmpty(row.FinishedAt),
			row.CreatedAt.Format(time.RFC3339),
			row.Error,
		})
	}
	cw.Flush()
}

func exportRow(r *ent.Run, suiteID int) wireapi.ExportRow {
	row := wireapi.ExportRow{
		RunID:      r.ID,
		SuiteID:    suiteID,
		Seed:       r.Seed,
		Status:     string(r.Status),
		Score:      r.Score,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		CreatedAt:  r.CreatedAt,
		Error:      r.Error,
	}
	if r.Edges.Submission != nil {
		row.SubmissionID = r.Edges.Submission.ID
		row.SubmissionName = r.Edges.Submission.Name
		row.Submitter = r.Edges.Submission.Submitter
	}
	if r.StartedAt != nil && r.FinishedAt != nil {
		ms := r.FinishedAt.Sub(*r.StartedAt).Milliseconds()
		row.DurationMs = &ms
	}
	return row
}

func floatOrEmpty(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

func intOrEmpty(n *int64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(*n, 10)
}

func timeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// slug makes a suite name safe for a download filename: lowercase, alnum and
// dashes only, collapsed.
func slug(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "suite"
	}
	return out
}
