package auth

import (
	"context"

	"github.com/januarsyah901/portofolio-api-go/internal/db"
)

// PGTokenStore implements TokenStore on the "ApiToken" table.
type PGTokenStore struct{ DB db.DB }

// FindByHash returns the token with the given hash, or db.ErrNotFound.
func (s PGTokenStore) FindByHash(ctx context.Context, hash string) (APIToken, error) {
	var t APIToken
	err := s.DB.QueryRow(ctx,
		`SELECT "id", "scope", "revokedAt" FROM "ApiToken" WHERE "tokenHash" = $1`, hash,
	).Scan(&t.ID, &t.Scope, &t.RevokedAt)
	return t, db.NotFound(err)
}

// TouchLastUsed sets lastUsedAt to now.
func (s PGTokenStore) TouchLastUsed(ctx context.Context, id int) error {
	_, err := s.DB.Exec(ctx, `UPDATE "ApiToken" SET "lastUsedAt" = now() AT TIME ZONE 'UTC' WHERE "id" = $1`, id)
	return err
}
