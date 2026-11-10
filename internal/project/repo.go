package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/januarsyah901/portofolio-api-go/internal/db"
)

// UnknownIDError is returned by Repo.Reorder when an id does not exist.
type UnknownIDError struct{ ID int }

func (e *UnknownIDError) Error() string { return fmt.Sprintf("project %d not found", e.ID) }

// Repo is the storage the handlers depend on.
type Repo interface {
	ListVisible(ctx context.Context) ([]Project, error)
	ListAll(ctx context.Context) ([]Project, error)
	GetVisibleBySlug(ctx context.Context, slug string) (Project, error)
	// SlugsWithPrefix returns existing slugs starting with prefix.
	SlugsWithPrefix(ctx context.Context, prefix string) ([]string, error)
	Create(ctx context.Context, in Input) (Project, error)
	// Update returns db.ErrNotFound when the id does not exist.
	Update(ctx context.Context, id int, in Input) (Project, error)
	// Delete returns db.ErrNotFound when the id does not exist.
	Delete(ctx context.Context, id int) error
	// Reorder sets "order" = index+1 for every id atomically; an unknown id yields
	// *UnknownIDError and nothing is changed.
	Reorder(ctx context.Context, ids []int) error
}

type pgRepo struct{ pool *pgxpool.Pool }

func newPgRepo(pool *pgxpool.Pool) *pgRepo { return &pgRepo{pool: pool} }

const columns = `"id", "title", "description", "imageUrl", "liveUrl", "githubUrl",
COALESCE("tags", '{}'), "slug", "category", "status", "year", "role",
COALESCE("highlights", '{}'), "releaseUrl", "featured", "order", "visible",
"createdAt", "updatedAt"`

func scan(row pgx.Row) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.Title, &p.Description, &p.ImageURL, &p.LiveURL, &p.GithubURL,
		&p.Tags, &p.Slug, &p.Category, &p.Status, &p.Year, &p.Role,
		&p.Highlights, &p.ReleaseURL, &p.Featured, &p.Order, &p.Visible,
		&p.CreatedAt.Time, &p.UpdatedAt.Time)
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if p.Highlights == nil {
		p.Highlights = []string{}
	}
	return p, err
}

func (r *pgRepo) list(ctx context.Context, query string) ([]Project, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *pgRepo) ListVisible(ctx context.Context) ([]Project, error) {
	return r.list(ctx, `SELECT `+columns+` FROM "Project" WHERE "visible" = true
		ORDER BY "featured" DESC, "order" ASC, "id" ASC`)
}

func (r *pgRepo) ListAll(ctx context.Context) ([]Project, error) {
	return r.list(ctx, `SELECT `+columns+` FROM "Project" ORDER BY "order" ASC, "id" ASC`)
}

func (r *pgRepo) GetVisibleBySlug(ctx context.Context, slug string) (Project, error) {
	p, err := scan(r.pool.QueryRow(ctx,
		`SELECT `+columns+` FROM "Project" WHERE "slug" = $1 AND "visible" = true LIMIT 1`, slug))
	return p, db.NotFound(err)
}

func (r *pgRepo) SlugsWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT "slug" FROM "Project" WHERE starts_with("slug", $1)`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func derefOr(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

func (r *pgRepo) Create(ctx context.Context, in Input) (Project, error) {
	tags, highlights := []string{}, []string{}
	if in.Tags != nil {
		tags = *in.Tags
	}
	if in.Highlights != nil {
		highlights = *in.Highlights
	}
	order, visible, featured := 0, true, false
	if in.Order != nil {
		order = *in.Order
	}
	if in.Visible != nil {
		visible = *in.Visible
	}
	if in.Featured != nil {
		featured = *in.Featured
	}
	p, err := scan(r.pool.QueryRow(ctx, `INSERT INTO "Project"
		("title", "description", "imageUrl", "liveUrl", "githubUrl", "releaseUrl", "year", "role",
		 "slug", "category", "status", "tags", "highlights", "featured", "order", "visible",
		 "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
		 now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC')
		RETURNING `+columns,
		derefOr(in.Title, ""), derefOr(in.Description, ""),
		in.ImageURL.Val, in.LiveURL.Val, in.GithubURL.Val, in.ReleaseURL.Val, in.Year.Val, in.Role.Val,
		in.Slug.Val, derefOr(in.Category, "web"), derefOr(in.Status, "live"),
		tags, highlights, featured, order, visible))
	return p, err
}

func (r *pgRepo) Update(ctx context.Context, id int, in Input) (Project, error) {
	var sets []string
	var args []any
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, `"`+col+`" = $`+strconv.Itoa(len(args)))
	}
	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.Description != nil {
		add("description", *in.Description)
	}
	for _, t := range []struct {
		col string
		val Text
	}{
		{"slug", in.Slug}, {"imageUrl", in.ImageURL}, {"liveUrl", in.LiveURL},
		{"githubUrl", in.GithubURL}, {"releaseUrl", in.ReleaseURL}, {"year", in.Year}, {"role", in.Role},
	} {
		if t.val.Set {
			add(t.col, t.val.Val)
		}
	}
	if in.Category != nil {
		add("category", *in.Category)
	}
	if in.Status != nil {
		add("status", *in.Status)
	}
	if in.Tags != nil {
		add("tags", *in.Tags)
	}
	if in.Highlights != nil {
		add("highlights", *in.Highlights)
	}
	if in.Featured != nil {
		add("featured", *in.Featured)
	}
	if in.Order != nil {
		add("order", *in.Order)
	}
	if in.Visible != nil {
		add("visible", *in.Visible)
	}
	sets = append(sets, `"updatedAt" = now() AT TIME ZONE 'UTC'`)
	args = append(args, id)
	p, err := scan(r.pool.QueryRow(ctx,
		`UPDATE "Project" SET `+strings.Join(sets, ", ")+
			` WHERE "id" = $`+strconv.Itoa(len(args))+` RETURNING `+columns, args...))
	return p, db.NotFound(err)
}

func (r *pgRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM "Project" WHERE "id" = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (r *pgRepo) Reorder(ctx context.Context, ids []int) error {
	return db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		for i, id := range ids {
			tag, err := tx.Exec(ctx,
				`UPDATE "Project" SET "order" = $1, "updatedAt" = now() AT TIME ZONE 'UTC' WHERE "id" = $2`,
				i+1, id)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return &UnknownIDError{ID: id}
			}
		}
		return nil
	})
}
