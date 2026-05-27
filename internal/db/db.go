// Package db opens the Ent client and runs schema bootstrap.
//
// DSN scheme selects the driver:
//   - sqlite://<path>[?<params>]   — uses modernc.org/sqlite (CGO-free)
//   - postgres://...               — uses jackc/pgx
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/nnamon/ctf-koth-evals-template/ent"
)

type Config struct {
	DSN string
}

// Open opens a database connection, runs schema migration, and returns the
// Ent client plus the resolved dialect. The caller owns Close.
func Open(ctx context.Context, cfg Config) (*ent.Client, string, error) {
	driver, dia, dsn, err := resolve(cfg.DSN)
	if err != nil {
		return nil, "", err
	}

	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", driver, err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, "", fmt.Errorf("ping %s: %w", driver, err)
	}

	drv := entsql.OpenDB(dia, sqlDB)
	client := ent.NewClient(ent.Driver(drv))

	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, "", fmt.Errorf("schema create: %w", err)
	}

	return client, dia, nil
}

func resolve(dsn string) (driver, dia, conn string, err error) {
	switch {
	case strings.HasPrefix(dsn, "sqlite://"):
		path := strings.TrimPrefix(dsn, "sqlite://")
		if path == "" {
			return "", "", "", fmt.Errorf("sqlite DSN missing path")
		}
		return "sqlite", dialect.SQLite, path, nil
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return "pgx", dialect.Postgres, dsn, nil
	default:
		return "", "", "", fmt.Errorf("unrecognized DSN scheme in %q (want sqlite:// or postgres://)", dsn)
	}
}
