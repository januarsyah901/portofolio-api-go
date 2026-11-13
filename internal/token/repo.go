package token

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

// ErrLimitReached means auth.MaxActiveTokens active tokens already exist.
var ErrLimitReached = errors.New("active token limit reached")

// Token is the public projection of an "ApiToken" row; the hash is never exposed.
type Token struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scope      string     `json:"scope"`
	CreatedAt  time.Time  `json:"-"`
	LastUsedAt *time.Time `json:"-"`
	RevokedAt  *time.Time `json:"-"`
}

// jsTime formats like JS Date.toISOString(): UTC, exactly 3 fractional digits.
const jsTime = "2006-01-02T15:04:05.000Z"

func fmtTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(jsTime)
	return &s
}

// MarshalJSON emits the public fields; timestamps are UTC (Prisma columns are
// TIMESTAMP WITHOUT TIME ZONE holding UTC).
func (t Token) MarshalJSON() ([]byte, error) {
	created := t.CreatedAt.UTC().Format(jsTime)
	return json.Marshal(struct {
		ID         int     `json:"id"`
		Name       string  `json:"name"`
		Prefix     string  `json:"prefix"`
		Scope      string  `json:"scope"`
		CreatedAt  string  `json:"createdAt"`
		LastUsedAt *string `json:"lastUsedAt"`
		RevokedAt  *string `json:"revokedAt"`
	}{t.ID, t.Name, t.Prefix, t.Scope, created, fmtTime(t.LastUsedAt), fmtTime(t.RevokedAt)})
}

// Repo is the persistence port used by Handler.
type Repo interface {
	List(ctx context.Context) ([]Token, error)
	// Create inserts a token unless the active limit is reached (ErrLimitReached).
	Create(ctx context.Context, name, hash, prefix, scope string) (Token, error)
	// Revoke reports whether an active token with that id was revoked.
	Revoke(ctx context.Context, id int) (bool, error)
}

type pgRepo struct{ pool *pgxpool.Pool }

const publicCols = `"id", "name", "prefix", "scope", "createdAt", "lastUsedAt", "revokedAt"`

func scanToken(row pgx.Row) (Token, error) {
	var t Token
	err := row.Scan(&t.ID, &t.Name, &t.Prefix, &t.Scope, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt)
	return t, err
}

func (r pgRepo) List(ctx context.Context) ([]Token, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+publicCols+` FROM "ApiToken" ORDER BY "createdAt" DESC, "id" DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// tokenLockKey is an arbitrary constant namespacing the advisory lock that
// serialises the count-then-insert critical section.
const tokenLockKey = 0x70665f746f6b656e

func (r pgRepo) Create(ctx context.Context, name, hash, prefix, scope string) (Token, error) {
	var out Token
	err := db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(tokenLockKey)); err != nil {
			return err
		}
		var active int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM "ApiToken" WHERE "revokedAt" IS NULL`).Scan(&active); err != nil {
			return err
		}
		if active >= auth.MaxActiveTokens {
			return ErrLimitReached
		}
		t, err := scanToken(tx.QueryRow(ctx,
			`INSERT INTO "ApiToken" ("name", "tokenHash", "prefix", "scope", "createdAt") VALUES ($1, $2, $3, $4, now() AT TIME ZONE 'UTC') RETURNING `+publicCols,
			name, hash, prefix, scope))
		out = t
		return err
	})
	return out, err
}

func (r pgRepo) Revoke(ctx context.Context, id int) (bool, error) {
	if id < 1 || id > math.MaxInt32 { // outside the SERIAL range: cannot exist
		return false, nil
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE "ApiToken" SET "revokedAt" = now() AT TIME ZONE 'UTC' WHERE "id" = $1 AND "revokedAt" IS NULL`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
