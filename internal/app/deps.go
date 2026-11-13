// Package app defines the dependencies handed to every module's Register function.
package app

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/config"
)

// Deps is everything a module needs to mount its routes.
type Deps struct {
	Config config.Config
	Pool   *pgxpool.Pool // nil in handler tests; modules must take repositories via interfaces
	JWT    *auth.JWT
	Guards auth.Guards
}
