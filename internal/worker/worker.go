// Package worker pulls runs from the API server, fetches the challenge
// bundle and submission artifact, lays out a working directory according to
// the platform's contract, invokes the per-challenge wrapper, and reports
// the outcome.
//
// The worker never talks to the database directly — all state changes go
// through the server's /internal API. The only persistent state held
// locally is the bundle cache.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nnamon/ctf-koth-evals-template/internal/bundle"
	"github.com/nnamon/ctf-koth-evals-template/internal/config"
	"github.com/nnamon/ctf-koth-evals-template/internal/wireapi"
	"github.com/nnamon/ctf-koth-evals-template/internal/workerclient"
)

type Deps struct {
	Cfg config.Config
}

func Run(ctx context.Context, deps Deps) error {
	cfg := deps.Cfg
	client := workerclient.New(workerclient.Options{
		BaseURL:  cfg.ServerURL,
		Token:    cfg.WorkerToken,
		WorkerID: cfg.WorkerID,
	})

	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}

	log.Printf("worker: id=%s server=%s concurrency=%d poll=%s", cfg.WorkerID, cfg.ServerURL, cfg.Concurrency, cfg.PollInterval)

	sem := make(chan struct{}, cfg.Concurrency)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("worker: draining %d in-flight runs", len(sem))
			for i := 0; i < cap(sem); i++ {
				sem <- struct{}{}
			}
			return nil
		case <-ticker.C:
			pull(ctx, client, cfg, sem)
		}
	}
}

func pull(ctx context.Context, client *workerclient.Client, cfg config.Config, sem chan struct{}) {
	for {
		select {
		case sem <- struct{}{}:
		default:
			return
		}
		claim, err := client.Claim(ctx)
		if err != nil {
			<-sem
			log.Printf("worker: claim error: %v", err)
			return
		}
		if claim == nil {
			<-sem
			return
		}
		go func(c *wireapi.ClaimResponse) {
			defer func() { <-sem }()
			if err := execute(ctx, client, cfg, c); err != nil {
				log.Printf("worker: run %d failed: %v", c.ID, err)
			}
		}(claim)
	}
}

func execute(ctx context.Context, client *workerclient.Client, cfg config.Config, claim *wireapi.ClaimResponse) error {
	bundlePath, err := ensureBundle(ctx, client, cfg, claim.Challenge.Version)
	if err != nil {
		return reportFailure(ctx, client, claim.ID, fmt.Errorf("ensure bundle: %w", err), nil)
	}

	artifact, err := client.FetchSubmission(ctx, claim.Submission.ID)
	if err != nil {
		return reportFailure(ctx, client, claim.ID, fmt.Errorf("fetch submission: %w", err), nil)
	}

	workdir := filepath.Join(cfg.CacheDir, "runs", fmt.Sprintf("%d", claim.ID))
	if err := os.RemoveAll(workdir); err != nil {
		return reportFailure(ctx, client, claim.ID, fmt.Errorf("clean workdir: %w", err), nil)
	}
	filename := submissionFilename(claim.Challenge.Manifest, claim.Submission.ArtifactName)
	if err := layoutWorkdir(workdir, filename, artifact, claim.Seed); err != nil {
		return reportFailure(ctx, client, claim.ID, fmt.Errorf("layout workdir: %w", err), nil)
	}

	wrapper := wrapperPath(bundlePath, claim.Challenge.Manifest)
	if _, err := os.Stat(wrapper); err != nil {
		return reportFailure(ctx, client, claim.ID, fmt.Errorf("wrapper not found: %w", err), nil)
	}

	timeout := time.Duration(claim.Suite.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, wrapper, "--workdir="+workdir)
	cmd.Env = append(os.Environ(), parametersToEnv(claim.Suite.Parameters)...)

	// Tee the subprocess streams: full output to logs/ in the workdir,
	// last-N-bytes tail to in-memory buffers that ride back in the result.
	stdoutTail := &tailWriter{limit: logTailLimit}
	stderrTail := &tailWriter{limit: logTailLimit}
	outWriters := []io.Writer{stdoutTail}
	errWriters := []io.Writer{stderrTail}
	if f, err := os.Create(filepath.Join(workdir, "logs", "stdout.log")); err == nil {
		defer f.Close()
		outWriters = append(outWriters, f)
	}
	if f, err := os.Create(filepath.Join(workdir, "logs", "stderr.log")); err == nil {
		defer f.Close()
		errWriters = append(errWriters, f)
	}
	cmd.Stdout = io.MultiWriter(outWriters...)
	cmd.Stderr = io.MultiWriter(errWriters...)

	startedAt := time.Now()
	runErr := cmd.Run()
	stdout := stdoutTail.String()
	stderr := stderrTail.String()

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return client.ReportResult(ctx, claim.ID, wireapi.ResultRequest{
			Status:    "timed_out",
			Error:     "wrapper exceeded suite timeout",
			StartedAt: &startedAt,
			Stdout:    stdout,
			Stderr:    stderr,
		})
	}
	if runErr != nil {
		return reportFailureLogs(ctx, client, claim.ID, fmt.Errorf("wrapper: %w", runErr), &startedAt, stdout, stderr)
	}

	result, score, err := readResult(filepath.Join(workdir, "result.json"))
	if err != nil {
		return reportFailureLogs(ctx, client, claim.ID, fmt.Errorf("read result: %w", err), &startedAt, stdout, stderr)
	}

	return client.ReportResult(ctx, claim.ID, wireapi.ResultRequest{
		Status:    "succeeded",
		Score:     &score,
		Result:    result,
		StartedAt: &startedAt,
		Stdout:    stdout,
		Stderr:    stderr,
	})
}

const logTailLimit = 32 * 1024 // bytes of stdout/stderr tail kept per run

// tailWriter keeps only the last `limit` bytes written — the tail of a
// stream, which is where errors usually surface.
type tailWriter struct {
	buf   []byte
	limit int
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.limit {
		t.buf = t.buf[len(t.buf)-t.limit:]
	}
	return len(p), nil
}

func (t *tailWriter) String() string { return string(t.buf) }

// ensureBundle returns the directory where the challenge bundle is
// extracted, fetching it from the server on cache miss.
func ensureBundle(ctx context.Context, client *workerclient.Client, cfg config.Config, version string) (string, error) {
	dest := filepath.Join(cfg.CacheDir, "challenges", version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	data, err := client.FetchBundle(ctx, version)
	if err != nil {
		return "", err
	}
	if got := bundle.Hash(data); got != version {
		return "", fmt.Errorf("bundle hash mismatch: server reported %s, got %s", version, got)
	}
	if err := bundle.Extract(data, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// parametersToEnv converts the suite's parameters map into CTF_PARAM_<UPPER>
// env var entries the wrapper can read. Nested maps and arrays are skipped —
// the contract is "scalar params per challenge". Challenge code wanting
// richer config should read it from a file the wrapper writes itself.
func parametersToEnv(params map[string]any) []string {
	if len(params) == 0 {
		return nil
	}
	out := make([]string, 0, len(params))
	for k, v := range params {
		switch v.(type) {
		case map[string]any, []any:
			continue
		}
		out = append(out, fmt.Sprintf("CTF_PARAM_%s=%v", strings.ToUpper(k), v))
	}
	return out
}

func wrapperPath(bundleDir string, manifest map[string]any) string {
	name := "wrapper"
	if w, ok := manifest["wrapper"].(string); ok && w != "" {
		name = w
	}
	return filepath.Join(bundleDir, name)
}

// submissionFilename returns the canonical filename the wrapper expects to
// find in the submission directory. The manifest's submission.file is
// authoritative — falling back to the uploaded artifact's name only when
// the manifest doesn't declare one.
func submissionFilename(manifest map[string]any, uploaded string) string {
	sub, ok := manifest["submission"].(map[string]any)
	if !ok {
		return uploaded
	}
	if name, ok := sub["file"].(string); ok && name != "" {
		return name
	}
	return uploaded
}

func layoutWorkdir(dir, artifactName string, artifact []byte, seed string) error {
	for _, sub := range []string{"submission", "inputs", "artifacts", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "submission", artifactName), artifact, 0o644); err != nil {
		return fmt.Errorf("write submission: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inputs", "seed.txt"), []byte(seed), 0o644); err != nil {
		return fmt.Errorf("write seed: %w", err)
	}
	return nil
}

func readResult(path string) (map[string]any, float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, 0, fmt.Errorf("invalid result.json: %w", err)
	}
	scoreRaw, ok := parsed["score"]
	if !ok {
		return nil, 0, fmt.Errorf("result.json missing score field")
	}
	score, ok := toFloat(scoreRaw)
	if !ok {
		return nil, 0, fmt.Errorf("result.json score is not a number: %v", scoreRaw)
	}
	return parsed, score, nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func reportFailure(ctx context.Context, client *workerclient.Client, runID int, cause error, startedAt *time.Time) error {
	_ = client.ReportResult(ctx, runID, wireapi.ResultRequest{
		Status:    "failed",
		Error:     cause.Error(),
		StartedAt: startedAt,
	})
	return cause
}

func reportFailureLogs(ctx context.Context, client *workerclient.Client, runID int, cause error, startedAt *time.Time, stdout, stderr string) error {
	_ = client.ReportResult(ctx, runID, wireapi.ResultRequest{
		Status:    "failed",
		Error:     cause.Error(),
		StartedAt: startedAt,
		Stdout:    stdout,
		Stderr:    stderr,
	})
	return cause
}
