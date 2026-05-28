package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/challenge"
	"github.com/nnamon/ctf-koth-evals-template/ent/run"
	"github.com/nnamon/ctf-koth-evals-template/internal/wireapi"
)

func (s *Deps) mountInternal(r chi.Router) {
	r.Route("/internal", func(in chi.Router) {
		in.Use(bearerAuth(s.Cfg.WorkerToken))
		in.Use(s.heartbeat)
		in.Post("/runs/claim", s.handleClaim)
		in.Get("/bundles/{hash}", s.handleBundle)
		in.Get("/submissions/{id}/artifact", s.handleArtifact)
		in.Get("/runs/{id}/status", s.handleRunStatus)
		in.Post("/runs/{id}/result", s.handleResult)
	})
}

// handleRunStatus is polled by a worker mid-execution to learn whether its
// run has been cancelled out from under it. Read-only, so it stays outside
// writeMu.
func (s *Deps) handleRunStatus(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	r, err := s.Client.Run.Query().
		Where(run.IDEQ(id)).
		Select(run.FieldStatus).
		Only(req.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(r.Status)})
}

func (s *Deps) handleClaim(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	workerID := req.Header.Get("X-Worker-Id")
	if workerID == "" {
		workerID = "unknown"
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.Client.Tx(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "begin tx: %v", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	pending, err := tx.Run.Query().
		Where(run.StatusEQ(run.StatusPending)).
		Order(ent.Desc(run.FieldPriority), ent.Asc(run.FieldCreatedAt)).
		WithSuite(func(q *ent.SuiteQuery) {
			q.WithChallenge()
		}).
		WithSubmission().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}

	now := time.Now()
	updated, err := tx.Run.UpdateOneID(pending.ID).
		SetStatus(run.StatusClaimed).
		SetWorkerID(workerID).
		SetClaimedAt(now).
		Save(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "update: %v", err)
		return
	}
	if err := tx.Commit(); err != nil {
		httpError(w, http.StatusInternalServerError, "commit: %v", err)
		return
	}
	s.notify("claim")

	resp := wireapi.ClaimResponse{
		ID:   updated.ID,
		Seed: updated.Seed,
		Suite: wireapi.Suite{
			ID:             pending.Edges.Suite.ID,
			TimeoutSeconds: pending.Edges.Suite.TimeoutSeconds,
			Parameters:     pending.Edges.Suite.Parameters,
		},
		Challenge: wireapi.Challenge{
			ID:       pending.Edges.Suite.Edges.Challenge.ID,
			Name:     pending.Edges.Suite.Edges.Challenge.Name,
			Version:  pending.Edges.Suite.Edges.Challenge.Version,
			Manifest: pending.Edges.Suite.Edges.Challenge.Manifest,
		},
		Submission: wireapi.Submission{
			ID:           pending.Edges.Submission.ID,
			ArtifactName: pending.Edges.Submission.ArtifactName,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Deps) handleBundle(w http.ResponseWriter, req *http.Request) {
	hash := chi.URLParam(req, "hash")
	if hash == "" {
		http.Error(w, "missing hash", http.StatusBadRequest)
		return
	}
	ch, err := s.Client.Challenge.Query().
		Where(challenge.VersionEQ(hash)).
		Select(challenge.FieldBundle, challenge.FieldBundleSize).
		First(req.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(ch.BundleSize, 10))
	_, _ = w.Write(ch.Bundle)
}

func (s *Deps) handleArtifact(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	sub, err := s.Client.Submission.Get(req.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "query: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(sub.ArtifactSize, 10))
	w.Header().Set("X-Artifact-Name", sub.ArtifactName)
	_, _ = w.Write(sub.Artifact)
}

func (s *Deps) handleResult(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body wireapi.ResultRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	status, err := parseTerminalStatus(body.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Sticky-cancel guard: if the run is already terminal, ignore the late
	// report. In particular a natural completion (succeeded/failed/timed_out)
	// must not override a user cancellation that landed while the subprocess
	// was being killed. A cancelled report on an already-cancelled run is
	// allowed through so its logs/timestamps still get recorded.
	cur, err := s.Client.Run.Query().Where(run.IDEQ(id)).Select(run.FieldStatus).Only(req.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "load run: %v", err)
		return
	}
	switch cur.Status {
	case run.StatusSucceeded, run.StatusFailed, run.StatusTimedOut:
		w.WriteHeader(http.StatusNoContent)
		return
	case run.StatusCancelled:
		if status != run.StatusCancelled {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	upd := s.Client.Run.UpdateOneID(id).
		SetStatus(status).
		SetFinishedAt(time.Now())
	if body.Score != nil {
		upd = upd.SetScore(*body.Score)
	}
	if body.Result != nil {
		upd = upd.SetResult(body.Result)
	}
	if body.Error != "" {
		upd = upd.SetError(body.Error)
	}
	if body.StartedAt != nil {
		upd = upd.SetStartedAt(*body.StartedAt)
	}
	if body.Stdout != "" {
		upd = upd.SetStdout(body.Stdout)
	}
	if body.Stderr != "" {
		upd = upd.SetStderr(body.Stderr)
	}

	if _, err := upd.Save(req.Context()); err != nil {
		if ent.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		httpError(w, http.StatusInternalServerError, "update: %v", err)
		return
	}

	if status == run.StatusSucceeded {
		// Already inside s.writeMu via the outer Lock; no extra serialization
		// needed here.
		if err := bumpRunsHandled(req.Context(), s.Client, req.Header.Get("X-Worker-Id")); err != nil {
			log.Printf("bumpRunsHandled: %v", err)
		}
	}

	s.notify("run")
	w.WriteHeader(http.StatusNoContent)
}

func parseTerminalStatus(s string) (run.Status, error) {
	switch run.Status(s) {
	case run.StatusSucceeded, run.StatusFailed, run.StatusTimedOut, run.StatusCancelled:
		return run.Status(s), nil
	}
	return "", fmt.Errorf("status must be one of succeeded|failed|timed_out|cancelled")
}

func bearerAuth(token string) func(http.Handler) http.Handler {
	expected := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if len(expected) == 0 {
				http.Error(w, "worker token not configured", http.StatusServiceUnavailable)
				return
			}
			auth := req.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(auth) < len(prefix) || auth[:len(prefix)] != prefix {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), expected) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func httpError(w http.ResponseWriter, status int, format string, args ...any) {
	http.Error(w, fmt.Sprintf(format, args...), status)
}

// silence unused-import lint if ctx not used directly
var _ = context.Background
