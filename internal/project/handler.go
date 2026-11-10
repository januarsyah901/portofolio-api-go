package project

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/januarsyah901/portofolio-api-go/internal/db"
	"github.com/januarsyah901/portofolio-api-go/internal/httpx"
)

// Messages identical to the Express version.
const (
	msgSlugTaken     = "Slug sudah dipakai proyek lain."
	msgNotFound      = "Proyek tidak ditemukan."
	msgInvalidID     = "ID proyek tidak valid."
	msgOrderedIDs    = "orderedIds harus berupa array dari ID proyek."
	msgReorderOK     = "Urutan proyek berhasil diperbarui."
	msgDeleteOK      = "Proyek berhasil dihapus."
	msgUnknownIDFmt  = "ID proyek tidak ditemukan: "
	maxPostgresInt32 = math.MaxInt32
)

// Handler serves the /api/projects endpoints.
type Handler struct{ repo Repo }

// NewHandler builds a Handler on top of repo.
func NewHandler(repo Repo) *Handler { return &Handler{repo: repo} }

func (h *Handler) listPublic(w http.ResponseWriter, r *http.Request) {
	ps, err := h.repo.ListVisible(r.Context())
	if err != nil {
		httpx.ServerError(w, r, "list public projects", err)
		return
	}
	httpx.JSON(w, http.StatusOK, ps)
}

func (h *Handler) listAdmin(w http.ResponseWriter, r *http.Request) {
	ps, err := h.repo.ListAll(r.Context())
	if err != nil {
		httpx.ServerError(w, r, "list admin projects", err)
		return
	}
	httpx.JSON(w, http.StatusOK, ps)
}

func (h *Handler) getBySlug(w http.ResponseWriter, r *http.Request) {
	p, err := h.repo.GetVisibleBySlug(r.Context(), r.PathValue("slug"))
	switch {
	case errors.Is(err, db.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, msgNotFound)
	case err != nil:
		httpx.ServerError(w, r, "get project by slug", err)
	default:
		httpx.JSON(w, http.StatusOK, p)
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
	if !in.Slug.Set {
		if base := Slugify(*in.Title); base != "" {
			taken, err := h.repo.SlugsWithPrefix(r.Context(), base)
			if err != nil {
				httpx.ServerError(w, r, "load taken slugs", err)
				return
			}
			slug := UniqueSlug(base, taken)
			in.Slug = Text{Set: true, Val: &slug}
		}
	}
	p, err := h.repo.Create(r.Context(), in)
	switch {
	case db.IsUniqueViolation(err):
		httpx.Error(w, http.StatusConflict, msgSlugTaken)
	case err != nil:
		httpx.ServerError(w, r, "create project", err)
	default:
		httpx.JSON(w, http.StatusCreated, p)
	}
}

// parseID mirrors Number(id) for a reorder entry (JSON number or numeric string) and
// requires a positive integer that fits an INTEGER column (PRD S9).
func parseID(raw json.RawMessage) (int, bool) {
	var n float64
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		n = x
	case string:
		f, err := strconv.ParseFloat(jsTrim(x), 64)
		if err != nil || strings.ContainsAny(x, "xXoObB_") {
			return 0, false
		}
		n = f
	default:
		return 0, false
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n < 1 || n > maxPostgresInt32 {
		return 0, false
	}
	return int(n), true
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) {
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	if !fields.IsArray("orderedIds") {
		httpx.Error(w, http.StatusBadRequest, msgOrderedIDs)
		return
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(fields["orderedIds"], &raws); err != nil {
		httpx.Error(w, http.StatusBadRequest, msgOrderedIDs)
		return
	}
	ids := make([]int, 0, len(raws))
	for _, raw := range raws {
		id, ok := parseID(raw)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, msgInvalidID)
			return
		}
		ids = append(ids, id)
	}
	err := h.repo.Reorder(r.Context(), ids)
	var unknown *UnknownIDError
	switch {
	case errors.As(err, &unknown):
		httpx.Error(w, http.StatusBadRequest, msgUnknownIDFmt+strconv.Itoa(unknown.ID)+".")
	case err != nil:
		httpx.ServerError(w, r, "reorder projects", err)
	default:
		httpx.JSON(w, http.StatusOK, httpx.Message{Message: msgReorderOK})
	}
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathInt(r, "id")
	if !ok {
		httpx.Error(w, http.StatusBadRequest, msgInvalidID)
		return
	}
	if id > maxPostgresInt32 || id < -maxPostgresInt32-1 {
		// The id column is int4: such a row cannot exist, and Postgres would reject the parameter.
		httpx.Error(w, http.StatusNotFound, msgNotFound)
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
	p, err := h.repo.Update(r.Context(), id, in)
	switch {
	case errors.Is(err, db.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, msgNotFound)
	case db.IsUniqueViolation(err):
		httpx.Error(w, http.StatusConflict, msgSlugTaken)
	case err != nil:
		httpx.ServerError(w, r, "update project", err)
	default:
		httpx.JSON(w, http.StatusOK, p)
	}
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathInt(r, "id")
	if !ok {
		httpx.Error(w, http.StatusBadRequest, msgInvalidID)
		return
	}
	if id > maxPostgresInt32 || id < -maxPostgresInt32-1 {
		// The id column is int4: such a row cannot exist, and Postgres would reject the parameter.
		httpx.Error(w, http.StatusNotFound, msgNotFound)
		return
	}
	err := h.repo.Delete(r.Context(), id)
	switch {
	case errors.Is(err, db.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, msgNotFound)
	case err != nil:
		httpx.ServerError(w, r, "delete project", err)
	default:
		httpx.JSON(w, http.StatusOK, httpx.Message{Message: msgDeleteOK})
	}
}
