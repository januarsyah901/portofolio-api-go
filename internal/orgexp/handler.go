package orgexp

import (
	"errors"
	"math"
	"net/http"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Handler serves /api/org-experience.
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
	in, msg := ParseInput(fields, Create)
	if msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	out, err := h.repo.Create(r.Context(), in.Apply(Defaults()))
	if err != nil {
		httpx.ServerError(w, r, "creating org experience", err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// id parses the path id. Ids beyond int32 cannot exist, so they are a 404
// (Express would have surfaced a Prisma error as 500).
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
	eid, ok := id(w, r)
	if !ok {
		return
	}
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	in, msg := ParseInput(fields, Update)
	if msg != "" {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	cur, err := h.repo.Get(r.Context(), eid)
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "updating org experience", err)
		return
	}
	out, err := h.repo.Update(r.Context(), eid, in.Apply(cur))
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "updating org experience", err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	eid, ok := id(w, r)
	if !ok {
		return
	}
	err := h.repo.Delete(r.Context(), eid)
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, MsgNotFound)
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "deleting org experience", err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Message{Message: MsgDeleted})
}
