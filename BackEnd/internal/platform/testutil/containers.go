package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// PostgresTestEnv bundles an isolated Testcontainers PostgreSQL instance with all up migrations applied.
type PostgresTestEnv struct {
	Container  *postgres.PostgresContainer
	ConnString string
	Pool       *pgxpool.Pool
	TxRunner   *db.PgxTxRunner
}

// RedisTestEnv bundles an isolated Testcontainers Redis instance.
type RedisTestEnv struct {
	Container *redis.RedisContainer
	URL       string
	Addr      string
}

// StartPostgres launches a postgres:16-alpine container, runs all BackEnd/db/migration/*.up.sql files,
// and registers cleanup on t.
func StartPostgres(t testing.TB) *PostgresTestEnv {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("hotel_test"),
		postgres.WithUsername("hotel"),
		postgres.WithPassword("hotel_test_pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres testcontainer: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("postgres connection string: %v", err)
	}

	pool, err := db.NewPool(ctx, db.PoolConfig{
		DatabaseURL:    connStr,
		MaxConns:       10,
		MinConns:       1,
		ConnectTimeout: 10 * time.Second,
	})
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("connect postgres testcontainer: %v", err)
	}

	migrationDir := MigrationsDir()
	if err := ApplyMigrations(ctx, pool, migrationDir); err != nil {
		pool.Close()
		_ = container.Terminate(context.Background())
		t.Fatalf("apply migrations from %s: %v", migrationDir, err)
	}

	t.Cleanup(func() {
		pool.Close()
		termCtx, termCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer termCancel()
		_ = container.Terminate(termCtx)
	})

	return &PostgresTestEnv{
		Container:  container,
		ConnString: connStr,
		Pool:       pool,
		TxRunner:   db.NewTxRunner(pool),
	}
}

// StartRedis launches a redis:7-alpine container and registers cleanup on t.
func StartRedis(t testing.TB) *RedisTestEnv {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis testcontainer: %v", err)
	}

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("redis connection string: %v", err)
	}
	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("redis endpoint: %v", err)
	}

	t.Cleanup(func() {
		termCtx, termCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer termCancel()
		_ = container.Terminate(termCtx)
	})

	return &RedisTestEnv{
		Container: container,
		URL:       connStr,
		Addr:      endpoint,
	}
}

// MigrationsDir resolves the absolute path to BackEnd/db/migration.
func MigrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "db/migration"
	}
	// file is BackEnd/internal/platform/testutil/containers.go -> BackEnd/db/migration
	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return filepath.Join(backendRoot, "db", "migration")
}

// ApplyMigrations executes every *.up.sql file in dir in sorted lexicographical order.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migration dir %s: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		sqlText := strings.TrimSpace(string(raw))
		if sqlText == "" {
			continue
		}
		if _, err := pool.Exec(ctx, sqlText); err != nil {
			return fmt.Errorf("exec migration %s: %w", name, err)
		}
	}
	return nil
}
