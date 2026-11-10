// Package testutil holds helpers for integration tests (build tag "integration").
package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB connects to TEST_DATABASE_URL and empties every table, so each test starts clean.
// It skips the test when TEST_DATABASE_URL is not set. Tests using it must not run in
// parallel with other packages' integration tests: run with `go test -p 1 -tags=integration`.
func DB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `TRUNCATE "Project", "Testimonial", "BlogVideo", "OrgExperience",
		"SiteContent", "AdminUser", "ApiToken" RESTART IDENTITY`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}
