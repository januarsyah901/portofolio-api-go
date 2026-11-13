// Package token serves /api/admin/tokens (API token management, JWT only).
package token

import (
	"net/http"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/auth"
)

// Register mounts the token routes; all require an admin JWT.
func Register(mux *http.ServeMux, d app.Deps) {
	var repo Repo
	if d.Pool != nil {
		repo = pgRepo{pool: d.Pool}
	}
	(&Handler{repo: repo}).routes(mux, d.Guards)
}

func (h *Handler) routes(mux *http.ServeMux, g auth.Guards) {
	mux.Handle("GET /api/admin/tokens", g.JWTOnly(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/admin/tokens", g.JWTOnly(http.HandlerFunc(h.create)))
	mux.Handle("DELETE /api/admin/tokens/{id}", g.JWTOnly(http.HandlerFunc(h.revoke)))
}
