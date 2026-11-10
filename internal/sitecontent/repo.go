package sitecontent

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the persistence the handler needs.
type Repo interface {
	// List returns every stored section keyed by name.
	List(ctx context.Context) (map[string]json.RawMessage, error)
	// Upsert replaces the section and returns the stored value.
	Upsert(ctx context.Context, key string, value []byte) (json.RawMessage, error)
	// Delete removes the section; a missing row is not an error.
	Delete(ctx context.Context, key string) error
}

type pgRepo struct{ pool *pgxpool.Pool }

func (r pgRepo) List(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT "key", "value" FROM "SiteContent"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = json.RawMessage(v)
	}
	return out, rows.Err()
}

func (r pgRepo) Upsert(ctx context.Context, key string, value []byte) (json.RawMessage, error) {
	var stored []byte
	err := r.pool.QueryRow(ctx, `
		INSERT INTO "SiteContent" ("key", "value", "updatedAt") VALUES ($1, $2::jsonb, now() AT TIME ZONE 'UTC')
		ON CONFLICT ("key") DO UPDATE SET "value" = EXCLUDED."value", "updatedAt" = now() AT TIME ZONE 'UTC'
		RETURNING "value"`, key, string(value)).Scan(&stored)
	return json.RawMessage(stored), err
}

func (r pgRepo) Delete(ctx context.Context, key string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM "SiteContent" WHERE "key" = $1`, key)
	return err
}
