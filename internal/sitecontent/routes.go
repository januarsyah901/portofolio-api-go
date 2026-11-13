// Package sitecontent implements: Site content (/api/site-content).
// See docs/PRD.md for the full contract.
package sitecontent

import (
	"net/http"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
)

// Register mounts this module's routes on mux.
func Register(mux *http.ServeMux, d app.Deps) {
	register(mux, d, pgRepo{pool: d.Pool})
}

func register(mux *http.ServeMux, d app.Deps, repo Repo) {
	h := &Handler{Repo: repo}
	mux.HandleFunc("GET /api/site-content", h.list)
	mux.HandleFunc("GET /api/site-content/{$}", h.list)
	mux.Handle("PUT /api/site-content/{key}", d.Guards.AdminOrWrite(http.HandlerFunc(h.put)))
	mux.Handle("DELETE /api/site-content/{key}", d.Guards.AdminOrWrite(http.HandlerFunc(h.delete)))
}
