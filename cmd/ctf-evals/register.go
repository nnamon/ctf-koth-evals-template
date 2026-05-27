package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nnamon/ctf-koth-evals-template/ent"
	"github.com/nnamon/ctf-koth-evals-template/ent/challenge"
	"github.com/nnamon/ctf-koth-evals-template/internal/bundle"
)

// registerChallenge packages the directory at dir, hashes it, and inserts a
// Challenge row keyed by (name, version=hash). If a row with the same name
// and version already exists, the call is a no-op.
func registerChallenge(ctx context.Context, client *ent.Client, dir string) error {
	manifest, err := readManifest(dir)
	if err != nil {
		return err
	}
	name, _ := manifest["name"].(string)
	if name == "" {
		return fmt.Errorf("manifest.json is missing a non-empty %q field", "name")
	}
	description, _ := manifest["description"].(string)

	data, version, err := bundle.Pack(dir)
	if err != nil {
		return fmt.Errorf("pack: %w", err)
	}

	existing, err := client.Challenge.Query().
		Where(challenge.NameEQ(name), challenge.VersionEQ(version)).
		Only(ctx)
	if err == nil {
		fmt.Printf("already registered: name=%s version=%s id=%d size=%d\n", name, version, existing.ID, existing.BundleSize)
		return nil
	}
	if !ent.IsNotFound(err) {
		return fmt.Errorf("lookup: %w", err)
	}

	created, err := client.Challenge.Create().
		SetName(name).
		SetVersion(version).
		SetDescription(description).
		SetManifest(manifest).
		SetBundle(data).
		SetBundleSize(int64(len(data))).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	fmt.Printf("registered: name=%s version=%s id=%d size=%d\n", name, version, created.ID, created.BundleSize)
	return nil
}

func readManifest(dir string) (map[string]any, error) {
	path := filepath.Join(dir, "manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return manifest, nil
}
