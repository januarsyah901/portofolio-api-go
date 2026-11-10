// Package upload implements: Image and CV uploads (/api/admin/upload) plus static files. Owner: worker M3.
// See docs/PRD.md for the full contract.
package upload

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
)

// Register mounts this module's routes on mux.
func Register(mux *http.ServeMux, d app.Deps) {
	publicDir := d.Config.PublicDir
	uploadsDir := filepath.Join(publicDir, "assets", "uploads")
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		slog.Error("create uploads dir", "dir", uploadsDir, "err", err)
	}
	h := &Handler{PublicDir: publicDir}

	mux.Handle("POST /api/admin/upload/image", d.Guards.JWTOnly(http.HandlerFunc(h.image)))
	mux.Handle("POST /api/admin/upload/cv", d.Guards.JWTOnly(http.HandlerFunc(h.cv)))

	mux.HandleFunc("GET /assets/uploads/{file...}", h.serveUpload)
	mux.HandleFunc("GET /cv.pdf", h.serveCV)
}
