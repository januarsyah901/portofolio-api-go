package blog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

// Video mirrors the Prisma BlogVideo model.
type Video struct {
	ID        int       `json:"id"`
	VideoID   string    `json:"videoId"`
	Caption   string    `json:"caption"`
	Tags      []string  `json:"tags"`
	Order     int       `json:"order"`
	Visible   bool      `json:"visible"`
	CreatedAt time.Time `json:"createdAt"`
}

// MarshalJSON renders createdAt like JS toISOString(): UTC, exactly 3 fractional digits.
func (m Video) MarshalJSON() ([]byte, error) {
	type alias Video
	return json.Marshal(struct {
		alias
		CreatedAt string `json:"createdAt"`
	}{alias(m), m.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")})
}

// Repo is the persistence port used by the handler.
type Repo interface {
	List(ctx context.Context, onlyVisible bool) ([]Video, error)
	Get(ctx context.Context, id int) (Video, error) // db.ErrNotFound when missing
	Create(ctx context.Context, in Video) (Video, error)
	Update(ctx context.Context, id int, in Video) (Video, error) // db.ErrNotFound when missing
	Delete(ctx context.Context, id int) error                    // db.ErrNotFound when missing
}

type pgRepo struct{ db db.DB }

func newPgRepo(d db.DB) *pgRepo { return &pgRepo{db: d} }

const cols = `"id", "videoId", "caption", COALESCE("tags", '{}'), "order", "visible", "createdAt"`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Video, error) {
	var v Video
	err := s.Scan(&v.ID, &v.VideoID, &v.Caption, &v.Tags, &v.Order, &v.Visible, &v.CreatedAt)
	if v.Tags == nil {
		v.Tags = []string{}
	}
	return v, err
}

func (r *pgRepo) List(ctx context.Context, onlyVisible bool) ([]Video, error) {
	q := `SELECT ` + cols + ` FROM "BlogVideo"`
	if onlyVisible {
		q += ` WHERE "visible" = true`
	}
	q += ` ORDER BY "order" ASC, "id" ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Video{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *pgRepo) Get(ctx context.Context, id int) (Video, error) {
	v, err := scan(r.db.QueryRow(ctx, `SELECT `+cols+` FROM "BlogVideo" WHERE "id" = $1`, id))
	return v, db.NotFound(err)
}

func (r *pgRepo) Create(ctx context.Context, in Video) (Video, error) {
	return scan(r.db.QueryRow(ctx,
		`INSERT INTO "BlogVideo" ("createdAt", "videoId", "caption", "tags", "order", "visible")
		 VALUES (now() AT TIME ZONE 'UTC', $1, $2, $3, $4, $5) RETURNING `+cols,
		in.VideoID, in.Caption, in.Tags, in.Order, in.Visible))
}

func (r *pgRepo) Update(ctx context.Context, id int, in Video) (Video, error) {
	v, err := scan(r.db.QueryRow(ctx,
		`UPDATE "BlogVideo" SET "videoId" = $2, "caption" = $3, "tags" = $4, "order" = $5, "visible" = $6
		 WHERE "id" = $1 RETURNING `+cols,
		id, in.VideoID, in.Caption, in.Tags, in.Order, in.Visible))
	return v, db.NotFound(err)
}

func (r *pgRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM "BlogVideo" WHERE "id" = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}
