package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/challenge"
)

// seedDemo creates a Suite + Submission + N Runs against an already-registered
// challenge. Intended for the end-to-end smoke test; not a long-term API.
func seedDemo(ctx context.Context, client *ent.Client, name, pattern string, runs int) error {
	ch, err := client.Challenge.Query().
		Where(challenge.NameEQ(name)).
		Order(ent.Desc(challenge.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		return fmt.Errorf("lookup challenge %q: %w", name, err)
	}

	seeds := make([]string, runs)
	for i := range seeds {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return err
		}
		seeds[i] = hex.EncodeToString(b[:])
	}

	suite, err := client.Suite.Create().
		SetName(fmt.Sprintf("%s-demo", name)).
		SetChallenge(ch).
		SetSeeds(seeds).
		SetTimeoutSeconds(30).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create suite: %w", err)
	}

	sub, err := client.Submission.Create().
		SetName("demo").
		SetSubmitter("demo").
		SetArtifactName("pattern.txt").
		SetArtifact([]byte(pattern)).
		SetArtifactSize(int64(len(pattern))).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create submission: %w", err)
	}

	for _, seed := range seeds {
		if _, err := client.Run.Create().
			SetSeed(seed).
			SetSuite(suite).
			SetSubmission(sub).
			Save(ctx); err != nil {
			return fmt.Errorf("create run: %w", err)
		}
	}

	fmt.Printf("seeded: suite=%d submission=%d runs=%d pattern=%q\n", suite.ID, sub.ID, runs, pattern)
	return nil
}
