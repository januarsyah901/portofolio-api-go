package sitecontent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Handler serves /api/site-content.
type Handler struct{ Repo Repo }

func unknownKey(w http.ResponseWriter, key string) {
	httpx.Error(w, http.StatusNotFound,
		fmt.Sprintf(`Section "%s" tidak dikenal. Pilihan: %s.`, key, strings.Join(Keys, ", ")))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	all, err := h.Repo.List(r.Context())
	if err != nil {
		httpx.ServerError(w, r, "fetching site content", err)
		return
	}
	if all == nil {
		all = map[string]json.RawMessage{}
	}
	httpx.JSON(w, http.StatusOK, all)
}

// readBody returns the decoded JSON body of any shape (httpx.DecodeObject would turn
// arrays/strings into {}, but the validators must report "harus berupa objek").
func readBody(w http.ResponseWriter, r *http.Request) (any, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpx.MaxJSONBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, httpx.MsgBodyTooLarge)
		} else {
			httpx.Error(w, http.StatusBadRequest, httpx.MsgInvalidJSON)
		}
		return nil, false
	}
	if strings.TrimSpace(string(body)) == "" {
		return map[string]any{}, true // Express: missing body parses as {}
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		httpx.Error(w, http.StatusBadRequest, httpx.MsgInvalidJSON)
		return nil, false
	}
	return v, true
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !IsKey(key) {
		unknownKey(w, key)
		return
	}
	raw, ok := readBody(w, r)
	if !ok {
		return
	}
	value, msg, ok := Validate(key, raw)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, msg)
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		httpx.ServerError(w, r, "saving site content", err)
		return
	}
	stored, err := h.Repo.Upsert(r.Context(), key, encoded)
	if err != nil {
		httpx.ServerError(w, r, "saving site content", err)
		return
	}
	httpx.JSON(w, http.StatusOK, struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}{key, stored})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !IsKey(key) {
		unknownKey(w, key)
		return
	}
	if err := h.Repo.Delete(r.Context(), key); err != nil {
		httpx.ServerError(w, r, "deleting site content", err)
		return
	}
	httpx.Error(w, http.StatusOK, fmt.Sprintf(`Section "%s" dihapus. Website memakai teks bawaan.`, key))
}
