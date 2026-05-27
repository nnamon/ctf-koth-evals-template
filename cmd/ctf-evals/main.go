// Command ctf-evals is a single binary with two modes:
//
//	ctf-evals serve   — HTTP API + UI
//	ctf-evals worker  — claims pending runs from the DB and executes them
//
// Configuration is read from environment variables; see internal/config.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/internal/config"
	"github.com/nnamon/ctf-koth-evals-template/internal/db"
	"github.com/nnamon/ctf-koth-evals-template/internal/server"
	"github.com/nnamon/ctf-koth-evals-template/internal/worker"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch os.Args[1] {
	case "serve":
		if cfg.Password == "" {
			fmt.Fprintln(os.Stderr, "serve: CTF_EVALS_PASSWORD is required")
			os.Exit(1)
		}
		if cfg.WorkerToken == "" {
			fmt.Fprintln(os.Stderr, "serve: CTF_EVALS_WORKER_TOKEN is required (used to authenticate workers)")
			os.Exit(1)
		}
		client := openDBOrExit(ctx, cfg.DSN)
		defer client.Close()
		if err := server.Run(ctx, &server.Deps{Cfg: cfg, Client: client}); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
	case "worker":
		if cfg.WorkerToken == "" {
			fmt.Fprintln(os.Stderr, "worker: CTF_EVALS_WORKER_TOKEN is required")
			os.Exit(1)
		}
		if err := worker.Run(ctx, worker.Deps{Cfg: cfg}); err != nil {
			fmt.Fprintf(os.Stderr, "worker: %v\n", err)
			os.Exit(1)
		}
	case "register-challenge":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: ctf-evals register-challenge <dir>")
			os.Exit(2)
		}
		client := openDBOrExit(ctx, cfg.DSN)
		defer client.Close()
		if err := registerChallenge(ctx, client, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "register-challenge: %v\n", err)
			os.Exit(1)
		}
	case "seed-demo":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: ctf-evals seed-demo <challenge> <pattern> [runs]")
			os.Exit(2)
		}
		runs := 5
		if len(os.Args) >= 5 {
			n, err := strconv.Atoi(os.Args[4])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "runs must be a positive int")
				os.Exit(2)
			}
			runs = n
		}
		client := openDBOrExit(ctx, cfg.DSN)
		defer client.Close()
		if err := seedDemo(ctx, client, os.Args[2], os.Args[3], runs); err != nil {
			fmt.Fprintf(os.Stderr, "seed-demo: %v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ctf-evals <serve|worker|register-challenge>")
}

func openDBOrExit(ctx context.Context, dsn string) *ent.Client {
	client, _, err := db.Open(ctx, db.Config{DSN: dsn})
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	return client
}
