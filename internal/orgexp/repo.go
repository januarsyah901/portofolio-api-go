package orgexp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

// Entry mirrors the Prisma OrgExperience model.
type Entry struct {
	ID           int       `json:"id"`
	Period       string    `json:"period"`
	Organization string    `json:"organization"`
	Role         string    `json:"role"`
	Description  string    `json:"description"`
	Highlights   []string  `json:"highlights"`
	Type         string    `json:"type"`
	Logo         *string   `json:"logo"`
	Order        int       `json:"order"`
	Visible      bool      `json:"visible"`
	CreatedAt    time.Time `json:"createdAt"`
}

// MarshalJSON renders createdAt like JS toISOString(): UTC, exactly 3 fractional digits.
func (m Entry) MarshalJSON() ([]byte, error) {
	type alias Entry
	return json.Marshal(struct {
		alias
		CreatedAt string `json:"createdAt"`
	}{alias(m), m.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")})
}

// Repo is the persistence port used by the handler.
type Repo interface {
	List(ctx context.Context, onlyVisible bool) ([]Entry, error)
	Get(ctx context.Context, id int) (Entry, error) // db.ErrNotFound when missing
	Create(ctx context.Context, in Entry) (Entry, error)
	Update(ctx context.Context, id int, in Entry) (Entry, error) // db.ErrNotFound when missing
	Delete(ctx context.Context, id int) error                    // db.ErrNotFound when missing
}

type pgRepo struct{ db db.DB }

func newPgRepo(d db.DB) *pgRepo { return &pgRepo{db: d} }

const cols = `"id", "period", "organization", "role", "description", COALESCE("highlights", '{}'),
	"type", "logo", "order", "visible", "createdAt"`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Entry, error) {
	var e Entry
	err := s.Scan(&e.ID, &e.Period, &e.Organization, &e.Role, &e.Description, &e.Highlights,
		&e.Type, &e.Logo, &e.Order, &e.Visible, &e.CreatedAt)
	if e.Highlights == nil {
		e.Highlights = []string{}
	}
	return e, err
}

func (r *pgRepo) List(ctx context.Context, onlyVisible bool) ([]Entry, error) {
	q := `SELECT ` + cols + ` FROM "OrgExperience"`
	if onlyVisible {
		q += ` WHERE "visible" = true`
	}
	q += ` ORDER BY "order" ASC, "id" ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *pgRepo) Get(ctx context.Context, id int) (Entry, error) {
	e, err := scan(r.db.QueryRow(ctx, `SELECT `+cols+` FROM "OrgExperience" WHERE "id" = $1`, id))
	return e, db.NotFound(err)
}

func (r *pgRepo) Create(ctx context.Context, in Entry) (Entry, error) {
	return scan(r.db.QueryRow(ctx,
		`INSERT INTO "OrgExperience" ("createdAt", "period", "organization", "role", "description", "highlights",
		 "type", "logo", "order", "visible") VALUES (now() AT TIME ZONE 'UTC', $1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING `+cols,
		in.Period, in.Organization, in.Role, in.Description, in.Highlights, in.Type, in.Logo, in.Order, in.Visible))
}

func (r *pgRepo) Update(ctx context.Context, id int, in Entry) (Entry, error) {
	e, err := scan(r.db.QueryRow(ctx,
		`UPDATE "OrgExperience" SET "period" = $2, "organization" = $3, "role" = $4, "description" = $5,
		 "highlights" = $6, "type" = $7, "logo" = $8, "order" = $9, "visible" = $10
		 WHERE "id" = $1 RETURNING `+cols,
		id, in.Period, in.Organization, in.Role, in.Description, in.Highlights, in.Type, in.Logo, in.Order, in.Visible))
	return e, db.NotFound(err)
}

func (r *pgRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM "OrgExperience" WHERE "id" = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}
