// Package authapi serves /api/auth/* (login, me, google-verify).
package authapi

import (
	"net/http"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
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
	rl := newLimiter()
	rl.ip = httpx.ClientIP{Trusted: d.Config.TrustedProxies}.Resolve
	h.routes(mux, d.Guards, rl)
}

func (h *Handler) routes(mux *http.ServeMux, g auth.Guards, rl *limiter) {
	mux.Handle("POST /api/auth/login", rl.middleware(http.HandlerFunc(h.login)))
	mux.Handle("GET /api/auth/me", g.JWTOnly(http.HandlerFunc(h.me)))
	mux.Handle("POST /api/auth/google-verify", rl.middleware(http.HandlerFunc(h.googleVerify)))
}
