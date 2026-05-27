// Package config centralises env-driven configuration for serve + worker modes.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DSN           string
	Addr          string
	Password      string
	WorkerToken   string
	ServerURL     string
	CacheDir      string
	WorkerID      string
	PollInterval  time.Duration
	Concurrency   int
	SweepInterval time.Duration
	WorkerTimeout time.Duration
	MaxUploadMB   int
}

func Load() (Config, error) {
	cfg := Config{
		DSN:         env("CTF_EVALS_DB", "sqlite://./data/ctf-evals.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"),
		Addr:        env("CTF_EVALS_ADDR", ":8080"),
		Password:    os.Getenv("CTF_EVALS_PASSWORD"),
		WorkerToken: os.Getenv("CTF_EVALS_WORKER_TOKEN"),
		ServerURL:   env("CTF_EVALS_SERVER_URL", "http://localhost:8080"),
		CacheDir:    env("CTF_EVALS_CACHE_DIR", "./data/cache"),
		WorkerID:    env("CTF_EVALS_WORKER_ID", hostnameOrRandom()),
	}

	interval, err := time.ParseDuration(env("CTF_EVALS_POLL_INTERVAL", "1s"))
	if err != nil {
		return Config{}, fmt.Errorf("CTF_EVALS_POLL_INTERVAL: %w", err)
	}
	cfg.PollInterval = interval

	conc, err := strconv.Atoi(env("CTF_EVALS_CONCURRENCY", "2"))
	if err != nil || conc < 1 {
		return Config{}, fmt.Errorf("CTF_EVALS_CONCURRENCY must be a positive int")
	}
	cfg.Concurrency = conc

	sweep, err := time.ParseDuration(env("CTF_EVALS_SWEEP_INTERVAL", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("CTF_EVALS_SWEEP_INTERVAL: %w", err)
	}
	cfg.SweepInterval = sweep

	timeout, err := time.ParseDuration(env("CTF_EVALS_WORKER_TIMEOUT", "30s"))
	if err != nil {
		return Config{}, fmt.Errorf("CTF_EVALS_WORKER_TIMEOUT: %w", err)
	}
	cfg.WorkerTimeout = timeout

	upload, err := strconv.Atoi(env("CTF_EVALS_MAX_UPLOAD_MB", "16"))
	if err != nil || upload < 1 {
		return Config{}, fmt.Errorf("CTF_EVALS_MAX_UPLOAD_MB must be a positive int")
	}
	cfg.MaxUploadMB = upload

	return cfg, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func hostnameOrRandom() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return fmt.Sprintf("worker-%d", os.Getpid())
}
