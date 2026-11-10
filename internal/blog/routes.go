// Package blog implements Blog videos (/api/blog).
// See docs/PRD.md section 4.5 for the contract.
package blog

import (
	"net/http"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
)

// Register mounts this module's routes on mux.
func Register(mux *http.ServeMux, d app.Deps) {
	registerWith(mux, d, newPgRepo(d.Pool))
}

func registerWith(mux *http.ServeMux, d app.Deps, repo Repo) {
	h := NewHandler(repo)
	mux.HandleFunc("GET /api/blog", h.list(true, "fetching public blog videos"))
	mux.Handle("GET /api/blog/admin-list", d.Guards.AdminOrRead(h.list(false, "fetching admin blog videos")))
	mux.Handle("POST /api/blog", d.Guards.AdminOrWrite(http.HandlerFunc(h.create)))
	mux.Handle("PUT /api/blog/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/blog/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.delete)))
}
