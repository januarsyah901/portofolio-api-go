// Package orgexp implements Org experience (/api/org-experience).
// See docs/PRD.md section 4.6 for the contract.
package orgexp

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
	mux.HandleFunc("GET /api/org-experience", h.list(true, "fetching public org experience"))
	mux.Handle("GET /api/org-experience/admin-list", d.Guards.AdminOrRead(h.list(false, "fetching admin org experience")))
	mux.Handle("POST /api/org-experience", d.Guards.AdminOrWrite(http.HandlerFunc(h.create)))
	mux.Handle("PUT /api/org-experience/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/org-experience/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.delete)))
}
