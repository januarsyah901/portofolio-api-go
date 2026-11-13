package upload

import (
	"net/http"
	"os"
	"strings"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

func notFound(w http.ResponseWriter) { httpx.Error(w, http.StatusNotFound, httpx.MsgNotFound) }

// serveUpload serves one file from <PublicDir>/assets/uploads. Directories and anything
// that would leave that directory are a 404.
func (h *Handler) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if name == "" || strings.ContainsAny(name, "\\\x00") {
		notFound(w)
		return
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." || seg == "." || seg == "" {
			notFound(w)
			return
		}
	}
	h.serveFile(w, r, h.PublicDir+"/assets/uploads", name)
}

func (h *Handler) serveCV(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, h.PublicDir, "cv.pdf")
}

// serveFile opens name inside dir through os.Root, which refuses ".." and symlink escapes.
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, dir, name string) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		notFound(w)
		return
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(name)
	if err != nil {
		notFound(w)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		notFound(w)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
