// Package server assembles the HTTP handler: global middleware, health checks and every module.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/authapi"
	"github.com/mrfroglab/portofolio-api-go/internal/blog"
	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
	"github.com/mrfroglab/portofolio-api-go/internal/orgexp"
	"github.com/mrfroglab/portofolio-api-go/internal/project"
	"github.com/mrfroglab/portofolio-api-go/internal/sitecontent"
	"github.com/mrfroglab/portofolio-api-go/internal/testimonial"
	"github.com/mrfroglab/portofolio-api-go/internal/token"
	"github.com/mrfroglab/portofolio-api-go/internal/upload"
)

// New returns the fully wired API handler.
func New(d app.Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{
			"status":  "OK",
			"message": "Server portfolio berjalan dengan baik.",
		})
	})
	mux.HandleFunc("GET /api/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if d.Pool == nil {
			httpx.Error(w, http.StatusServiceUnavailable, "Database tidak tersedia.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "Database tidak tersedia.")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "OK"})
	})

	// Each module owns its routes; keep this list in the same order as docs/PRD.md §4.
	authapi.Register(mux, d)
	token.Register(mux, d)
	project.Register(mux, d)
	testimonial.Register(mux, d)
	blog.Register(mux, d)
	orgexp.Register(mux, d)
	sitecontent.Register(mux, d)
	upload.Register(mux, d)

	// Anything unmatched gets a JSON 404 instead of the mux's plain-text one.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusNotFound, httpx.MsgNotFound)
	})

	return httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.AccessLog,
		httpx.Recover,
		httpx.CORS(d.Config.CORSOrigins),
	)
}
