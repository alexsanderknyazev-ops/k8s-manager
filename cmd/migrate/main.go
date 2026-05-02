package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type migration struct {
	version int
	path    string
}

const advisoryLockKey int64 = 834512991337

func main() {
	action := flag.String("action", "up", "migration action: up|down|status")
	dsn := flag.String("dsn", os.Getenv("POSTGRES_DSN"), "postgres dsn")
	dir := flag.String("dir", "migrations", "migrations directory")
	timeout := flag.Duration("timeout", migrationTimeout(), "timeout per DB operation (e.g. 30s, 2m)")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "POSTGRES_DSN (or -dsn) is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer pool.Close()
	opCtx, opCancel := context.WithTimeout(context.Background(), *timeout)
	defer opCancel()
	if err := ensureSchemaMigrations(opCtx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "ensure schema_migrations:", err)
		os.Exit(1)
	}
	if err := acquireLock(opCtx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "acquire migration lock:", err)
		os.Exit(1)
	}
	defer func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unlockCancel()
		if err := releaseLock(unlockCtx, pool); err != nil {
			fmt.Fprintln(os.Stderr, "release migration lock:", err)
		}
	}()

	switch *action {
	case "up":
		if err := migrateUp(context.Background(), pool, *dir, *timeout); err != nil {
			fmt.Fprintln(os.Stderr, "migrate up:", err)
			os.Exit(1)
		}
	case "down":
		if err := migrateDown(context.Background(), pool, *dir, *timeout); err != nil {
			fmt.Fprintln(os.Stderr, "migrate down:", err)
			os.Exit(1)
		}
	case "status":
		if err := migrateStatus(opCtx, pool, *dir); err != nil {
			fmt.Fprintln(os.Stderr, "migrate status:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown action:", *action)
		os.Exit(1)
	}
}

func migrationTimeout() time.Duration {
	raw := os.Getenv("MIGRATE_TIMEOUT")
	if raw == "" {
		return 2 * time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 2 * time.Minute
	}
	return d
}

func acquireLock(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey)
	return err
}

func releaseLock(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey)
	return err
}

func ensureSchemaMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func listMigrations(dir, suffix string) ([]migration, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*"+suffix))
	if err != nil {
		return nil, err
	}
	out := make([]migration, 0, len(paths))
	for _, p := range paths {
		base := filepath.Base(p)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid migration filename: %s", base)
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid migration version in %s: %w", base, err)
		}
		out = append(out, migration{version: v, path: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vs := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vs[v] = true
	}
	return vs, rows.Err()
}

func migrateUp(ctx context.Context, pool *pgxpool.Pool, dir string, timeout time.Duration) error {
	migs, err := listMigrations(dir, ".up.sql")
	if err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		sqlBytes, err := os.ReadFile(m.path)
		if err != nil {
			return err
		}
		opCtx, cancel := context.WithTimeout(ctx, timeout)
		tx, err := pool.Begin(opCtx)
		if err != nil {
			cancel()
			return err
		}
		if _, err := tx.Exec(opCtx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(opCtx)
			cancel()
			return fmt.Errorf("apply %s: %w", m.path, err)
		}
		if _, err := tx.Exec(opCtx, `INSERT INTO schema_migrations(version) VALUES ($1)`, m.version); err != nil {
			_ = tx.Rollback(opCtx)
			cancel()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(opCtx); err != nil {
			cancel()
			return err
		}
		cancel()
		fmt.Printf("applied %d: %s\n", m.version, filepath.Base(m.path))
	}
	return nil
}

func migrateDown(ctx context.Context, pool *pgxpool.Pool, dir string, timeout time.Duration) error {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return errors.New("no applied migrations")
	}
	var v int
	if err := rows.Scan(&v); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%04d_", v))
	matches, err := filepath.Glob(path + "*.down.sql")
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("down migration for version %d not found", v)
	}
	if len(matches) > 1 {
		return fmt.Errorf("multiple down migrations for version %d: %v", v, matches)
	}
	sqlBytes, err := os.ReadFile(matches[0])
	if err != nil {
		return err
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tx, err := pool.Begin(opCtx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(opCtx, string(sqlBytes)); err != nil {
		_ = tx.Rollback(opCtx)
		return fmt.Errorf("apply down %s: %w", matches[0], err)
	}
	if _, err := tx.Exec(opCtx, `DELETE FROM schema_migrations WHERE version=$1`, v); err != nil {
		_ = tx.Rollback(opCtx)
		return err
	}
	if err := tx.Commit(opCtx); err != nil {
		return err
	}
	fmt.Printf("rolled back %d: %s\n", v, filepath.Base(matches[0]))
	return nil
}

func migrateStatus(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	migs, err := listMigrations(dir, ".up.sql")
	if err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	for _, m := range migs {
		state := "pending"
		if applied[m.version] {
			state = "applied"
		}
		fmt.Printf("%04d  %-7s  %s\n", m.version, state, filepath.Base(m.path))
	}
	return nil
}
