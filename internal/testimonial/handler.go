package testimonial

import (
	"errors"
	"math"
	"net/http"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Handler serves /api/testimonials.
type Handler struct{ repo Repo }

// NewHandler builds a Handler around a Repo.
func NewHandler(r Repo) *Handler { return &Handler{repo: r} }

func (h *Handler) list(visibleOnly bool, label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.repo.List(r.Context(), visibleOnly)
		if err != nil {
			httpx.ServerError(w, r, label, err)
			return
		}
		httpx.JSON(w, http.StatusOK, items)
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	in, msg := ParseCreate(fields)
	if msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	out, err := h.repo.Create(r.Context(), in)
	if err != nil {
		httpx.ServerError(w, r, "creating testimonial", err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// id parses the path id; ids beyond int32 can never exist, so they are reported as not found
// by the callers (Express would have surfaced a Prisma error as 500).
func id(w http.ResponseWriter, r *http.Request) (int, bool) {
	n, ok := httpx.PathInt(r, "id")
	if !ok {
		httpx.Error(w, http.StatusBadRequest, MsgBadID)
		return 0, false
	}
	if n > math.MaxInt32 || n < math.MinInt32 {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return 0, false
	}
	return n, true
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	tid, ok := id(w, r)
	if !ok {
		return
	}
	cur, err := h.repo.Get(r.Context(), tid)
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "updating testimonial", err)
		return
	}
	merged, msg := ApplyUpdate(fields, cur)
	if msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	out, err := h.repo.Update(r.Context(), tid, merged)
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "updating testimonial", err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tid, ok := id(w, r)
	if !ok {
		return
	}
	err := h.repo.Delete(r.Context(), tid)
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "deleting testimonial", err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Message{Message: MsgDeleted})
}
