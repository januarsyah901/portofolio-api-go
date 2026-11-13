package authapi

import (
	"context"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

// User is the public projection of an "AdminUser" row.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// Repo is the persistence port used by Handler.
type Repo interface {
	// FindByUsername returns the user and bcrypt hash; db.ErrNotFound when missing.
	FindByUsername(ctx context.Context, username string) (User, string, error)
	FindByID(ctx context.Context, id int) (User, error)
	// Create inserts a user; db.IsUniqueViolation(err) on duplicate usernames.
	Create(ctx context.Context, username, passwordHash string) (User, error)
}

type pgRepo struct{ db db.DB }

func (r pgRepo) FindByUsername(ctx context.Context, username string) (User, string, error) {
	var u User
	var hash string
	err := r.db.QueryRow(ctx,
		`SELECT "id", "username", "passwordHash" FROM "AdminUser" WHERE "username" = $1 LIMIT 1`, username,
	).Scan(&u.ID, &u.Username, &hash)
	return u, hash, db.NotFound(err)
}

func (r pgRepo) FindByID(ctx context.Context, id int) (User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`SELECT "id", "username" FROM "AdminUser" WHERE "id" = $1`, id,
	).Scan(&u.ID, &u.Username)
	return u, db.NotFound(err)
}

func (r pgRepo) Create(ctx context.Context, username, passwordHash string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`INSERT INTO "AdminUser" ("username", "passwordHash") VALUES ($1, $2) RETURNING "id", "username"`,
		username, passwordHash,
	).Scan(&u.ID, &u.Username)
	return u, err
}
