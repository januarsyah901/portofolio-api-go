// Package authapi serves /api/auth/* (login, me, google-verify).
package authapi

import (
	"net/http"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
	"github.com/januarsyah901/portofolio-api-go/internal/auth"
)

// Register mounts the auth routes. Login and google-verify are rate limited per client IP.
func Register(mux *http.ServeMux, d app.Deps) {
	var repo Repo
	if d.Pool != nil {
		repo = pgRepo{db: d.Pool}
	}
	h := &Handler{
		repo:      repo,
		jwt:       d.JWT,
		admins:    d.Config.AdminEmails,
		client:    &http.Client{Timeout: googleTimeout},
		googleURL: googleUserinfoURL,
	}
	h.routes(mux, d.Guards, newLimiter())
}

func (h *Handler) routes(mux *http.ServeMux, g auth.Guards, rl *limiter) {
	mux.Handle("POST /api/auth/login", rl.middleware(http.HandlerFunc(h.login)))
	mux.Handle("GET /api/auth/me", g.JWTOnly(http.HandlerFunc(h.me)))
	mux.Handle("POST /api/auth/google-verify", rl.middleware(http.HandlerFunc(h.googleVerify)))
}
