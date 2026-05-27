// Package server hosts the HTTP API plus the built SPA.
//
// Auth model is intentionally minimal: a single shared password gates every
// /api/* endpoint via HTTP Basic auth. The username is ignored; only the
// password is checked.
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/internal/config"
)

type Deps struct {
	Cfg    config.Config
	Client *ent.Client

	// writeMu serializes write-heavy paths on /internal/* so SQLite doesn't
	// SQLITE_BUSY itself under concurrent workers. On Postgres this is
	// unnecessary overhead but harmless; the bottleneck moves to the DB.
	writeMu sync.Mutex
}

func Run(ctx context.Context, deps *Deps) error {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	r.Route("/api", func(api chi.Router) {
		api.Use(basicAuth(deps.Cfg.Password))
		deps.mountPublic(api)
	})

	deps.mountInternal(r)

	spa := newSPAHandler(filepath.Join("web", "dist"))
	r.Handle("/*", spa)

	srv := &http.Server{
		Addr:              deps.Cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go deps.runSweeper(ctx, deps.Cfg.WorkerTimeout, deps.Cfg.SweepInterval)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("serve: listening on %s", deps.Cfg.Addr)
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func basicAuth(password string) func(http.Handler) http.Handler {
	expected := []byte(password)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, got, ok := req.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="ctf-evals"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// newSPAHandler serves files from dir, falling back to index.html for any
// path that doesn't resolve to a file — needed for client-side routing.
func newSPAHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := filepath.Join(dir, filepath.Clean(req.URL.Path))
		if info, err := stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, req)
			return
		}
		index := filepath.Join(dir, "index.html")
		if _, err := stat(index); err != nil {
			http.Error(w, fmt.Sprintf("SPA not built — run `npm run build` in web/ (looking for %s)", index), http.StatusNotFound)
			return
		}
		http.ServeFile(w, req, index)
	})
}
