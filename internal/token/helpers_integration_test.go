//go:build integration

package token

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
	"github.com/januarsyah901/portofolio-api-go/internal/auth"
)

func appDeps(pool *pgxpool.Pool, j *auth.JWT, g auth.Guards) app.Deps {
	return app.Deps{Pool: pool, JWT: j, Guards: g}
}
