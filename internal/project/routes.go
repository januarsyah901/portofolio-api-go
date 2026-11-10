// Package project implements Projects (/api/projects). See docs/PRD.md section 4.3.
package project

import (
	"net/http"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
)

// Register mounts this module's routes on mux.
func Register(mux *http.ServeMux, d app.Deps) {
	h := NewHandler(newPgRepo(d.Pool))
	mux.HandleFunc("GET /api/projects", h.listPublic)
	mux.Handle("GET /api/projects/admin-list", d.Guards.AdminOrRead(http.HandlerFunc(h.listAdmin)))
	mux.Handle("POST /api/projects", d.Guards.AdminOrWrite(http.HandlerFunc(h.create)))
	mux.Handle("PUT /api/projects/reorder", d.Guards.AdminOrWrite(http.HandlerFunc(h.reorder)))
	mux.HandleFunc("GET /api/projects/{slug}", h.getBySlug)
	mux.Handle("PUT /api/projects/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/projects/{id}", d.Guards.AdminOrWrite(http.HandlerFunc(h.delete)))
}
