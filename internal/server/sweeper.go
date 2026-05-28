package server

import (
	"context"
	"log"
	"time"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/run"
	"github.com/nnamon/ctf-koth-evals-template/ent/worker"
)

// runSweeper periodically reclaims runs whose claiming worker has stopped
// heart-beating. A worker's last_seen is bumped on every internal API call
// (including idle claim polls), so a healthy worker will keep its claimed
// runs locked. When last_seen ages past workerTimeout, the worker is
// treated as dead and any runs it claimed are returned to pending.
func (s *Deps) runSweeper(ctx context.Context, workerTimeout, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.writeMu.Lock()
			n, err := sweep(ctx, s.Client, workerTimeout)
			s.writeMu.Unlock()
			if err != nil {
				log.Printf("sweeper: %v", err)
			} else if n > 0 {
				log.Printf("sweeper: reclaimed %d orphaned run(s)", n)
				s.notify("runs")
			}
		}
	}
}

func sweep(ctx context.Context, client *ent.Client, workerTimeout time.Duration) (int, error) {
	cutoff := time.Now().Add(-workerTimeout)

	fresh, err := client.Worker.Query().
		Where(worker.LastSeenGT(cutoff)).
		Select(worker.FieldName).
		Strings(ctx)
	if err != nil {
		return 0, err
	}

	pred := []func(*ent.RunMutation) error{}
	_ = pred // placeholder; using builder predicates below

	upd := client.Run.Update().
		Where(run.StatusEQ(run.StatusClaimed)).
		SetStatus(run.StatusPending).
		SetWorkerID("").
		ClearClaimedAt()

	if len(fresh) > 0 {
		upd = upd.Where(run.WorkerIDNotIn(fresh...))
	}

	return upd.Save(ctx)
}
