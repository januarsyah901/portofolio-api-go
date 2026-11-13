package testimonial

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

// Testimonial mirrors the Prisma model (camelCase keys, nullable columns as pointers).
type Testimonial struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Company   *string   `json:"company"`
	AvatarURL *string   `json:"avatarUrl"`
	Text      string    `json:"text"`
	Rating    int       `json:"rating"`
	Visible   bool      `json:"visible"`
	CreatedAt time.Time `json:"createdAt"`
}

// MarshalJSON renders createdAt like JS toISOString(): UTC, exactly 3 fractional digits.
func (m Testimonial) MarshalJSON() ([]byte, error) {
	type alias Testimonial
	return json.Marshal(struct {
		alias
		CreatedAt string `json:"createdAt"`
	}{alias(m), m.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")})
}

// Repo is the persistence port used by the handler.
type Repo interface {
	List(ctx context.Context, onlyVisible bool) ([]Testimonial, error)
	Get(ctx context.Context, id int) (Testimonial, error) // db.ErrNotFound when missing
	Create(ctx context.Context, in Testimonial) (Testimonial, error)
	Update(ctx context.Context, id int, in Testimonial) (Testimonial, error) // db.ErrNotFound when missing
	Delete(ctx context.Context, id int) error                                // db.ErrNotFound when missing
}

type pgRepo struct{ db db.DB }

func newPgRepo(d db.DB) *pgRepo { return &pgRepo{db: d} }

const cols = `"id", "name", "role", "company", "avatarUrl", "text", "rating", "visible", "createdAt"`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Testimonial, error) {
	var t Testimonial
	err := s.Scan(&t.ID, &t.Name, &t.Role, &t.Company, &t.AvatarURL, &t.Text, &t.Rating, &t.Visible, &t.CreatedAt)
	return t, err
}

func (r *pgRepo) List(ctx context.Context, onlyVisible bool) ([]Testimonial, error) {
	q := `SELECT ` + cols + ` FROM "Testimonial"`
	if onlyVisible {
		q += ` WHERE "visible" = true`
	}
	q += ` ORDER BY "createdAt" DESC, "id" DESC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Testimonial{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *pgRepo) Get(ctx context.Context, id int) (Testimonial, error) {
	t, err := scan(r.db.QueryRow(ctx, `SELECT `+cols+` FROM "Testimonial" WHERE "id" = $1`, id))
	return t, db.NotFound(err)
}

func (r *pgRepo) Create(ctx context.Context, in Testimonial) (Testimonial, error) {
	return scan(r.db.QueryRow(ctx,
		`INSERT INTO "Testimonial" ("createdAt", "name", "role", "company", "avatarUrl", "text", "rating", "visible")
		 VALUES (now() AT TIME ZONE 'UTC', $1, $2, $3, $4, $5, $6, $7) RETURNING `+cols,
		in.Name, in.Role, in.Company, in.AvatarURL, in.Text, in.Rating, in.Visible))
}

func (r *pgRepo) Update(ctx context.Context, id int, in Testimonial) (Testimonial, error) {
	t, err := scan(r.db.QueryRow(ctx,
		`UPDATE "Testimonial" SET "name" = $2, "role" = $3, "company" = $4, "avatarUrl" = $5,
		 "text" = $6, "rating" = $7, "visible" = $8 WHERE "id" = $1 RETURNING `+cols,
		id, in.Name, in.Role, in.Company, in.AvatarURL, in.Text, in.Rating, in.Visible))
	return t, db.NotFound(err)
}

func (r *pgRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM "Testimonial" WHERE "id" = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}
