// Package testimonial implements Testimonials (/api/testimonials).
// See docs/PRD.md section 4.4 for the contract.
package testimonial

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
	mux.HandleFunc("GET /api/testimonials", h.list(true, "fetching public testimonials"))
	mux.Handle("GET /api/testimonials/admin-list", d.Guards.AdminOrRead(h.list(false, "fetching admin testimonials")))
	mux.Handle("POST /api/testimonials", d.Guards.AdminOrWrite(http.HandlerFunc(h.create)))
	mux.Handle("PUT /api/testimonials/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/testimonials/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.delete)))
}
