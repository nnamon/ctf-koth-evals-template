package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/worker"
)

// heartbeatTTL is how stale a worker's last_seen has to be before we bother
// hitting the DB to refresh it. Throttles the otherwise-per-request upsert,
// which dominated SQLite write contention under busy workers.
const heartbeatTTL = 5 * time.Second

var (
	hbCache   = map[string]time.Time{}
	hbCacheMu sync.Mutex
)

// heartbeat is a middleware that records the calling worker's last_seen.
// The actual DB upsert is throttled to once per heartbeatTTL per worker
// name — repeated polls within the window are no-ops.
func (s *Deps) heartbeat(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := req.Header.Get("X-Worker-Id")
		if id != "" && shouldHeartbeat(id) {
			s.writeMu.Lock()
			err := upsertWorker(req.Context(), s.Client, id)
			s.writeMu.Unlock()
			if err != nil {
				log.Printf("heartbeat: %v", err)
			}
		}
		next.ServeHTTP(w, req)
	})
}

func shouldHeartbeat(id string) bool {
	hbCacheMu.Lock()
	defer hbCacheMu.Unlock()
	now := time.Now()
	if last, ok := hbCache[id]; ok && now.Sub(last) < heartbeatTTL {
		return false
	}
	hbCache[id] = now
	return true
}

func upsertWorker(ctx context.Context, client *ent.Client, name string) error {
	now := time.Now()
	return client.Worker.Create().
		SetName(name).
		SetLastSeen(now).
		OnConflict(
			sql.ConflictColumns(worker.FieldName),
		).
		UpdateLastSeen().
		Exec(ctx)
}

func bumpRunsHandled(ctx context.Context, client *ent.Client, name string) error {
	if name == "" {
		return nil
	}
	_, err := client.Worker.Update().
		Where(worker.NameEQ(name)).
		AddRunsHandled(1).
		SetLastSeen(time.Now()).
		Save(ctx)
	return err
}
