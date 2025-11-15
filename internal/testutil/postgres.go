package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	var connStr string
	var container testcontainers.Container

	existingDB := os.Getenv("DATABASE_URL")
	useExistingDB := existingDB != ""

	if useExistingDB {
		connStr = existingDB
		t.Log("Using existing database from DATABASE_URL")
	} else {
		pgContainer, err := postgres.Run(ctx,
			"postgres:15-alpine",
			postgres.WithDatabase("testdb"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("postgres"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(5*time.Minute)),
		)
		if err != nil {
			t.Fatalf("Failed to start postgres container: %v", err)
		}

		container = pgContainer

		t.Cleanup(func() {
			if terminateErr := container.Terminate(ctx); terminateErr != nil {
				t.Fatalf("Failed to terminate container: %v", terminateErr)
			}
		})

		host, err := pgContainer.Host(ctx)
		if err != nil {
			t.Fatalf("Failed to get container host: %v", err)
		}

		port, err := pgContainer.MappedPort(ctx, "5432")
		if err != nil {
			t.Fatalf("Failed to get container port: %v", err)
		}

		connStr = fmt.Sprintf("postgres://postgres:postgres@%s:%s/testdb?sslmode=disable", host, port.Port())
		t.Logf("Using testcontainer database: %s", connStr)
	}

	if !useExistingDB {
		if err := runMigrations(connStr); err != nil {
			t.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		t.Log("Skipping migrations - using pre-migrated database")
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if !useExistingDB {
		cleanupTestData(ctx, pool, t)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func runMigrations(dbURL string) error {
	m, err := migrate.New(
		"file://../../migrations",
		dbURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Printf("Warning: failed to close migration source: %v\n", srcErr)
		}
		if dbErr != nil {
			fmt.Printf("Warning: failed to close migration database connection: %v\n", dbErr)
		}
	}()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		fmt.Printf("Warning: failed to run down migrations: %v\n", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func cleanupTestData(ctx context.Context, pool *pgxpool.Pool, t *testing.T) {
	query := `
                TRUNCATE TABLE pr_reviewers, pull_requests, users, teams 
                RESTART IDENTITY CASCADE
        `

	if _, err := pool.Exec(ctx, query); err != nil {
		t.Logf("Warning: failed to clean up test data: %v", err)
	}
}
