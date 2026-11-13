package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Handler serves /api/admin/tokens.
type Handler struct{ repo Repo }

// created is the 201 body: public fields plus the one-time plaintext token.
type created struct {
	Token
	Secret string
}

// MarshalJSON is explicit because the embedded Token's MarshalJSON would
// otherwise be promoted and drop the secret.
func (c created) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(c.Token)
	if err != nil {
		return nil, err
	}
	secret, err := json.Marshal(c.Secret)
	if err != nil {
		return nil, err
	}
	return append(base[:len(base)-1], []byte(`,"token":`+string(secret)+`}`)...), nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.repo.List(r.Context())
	if err != nil {
		httpx.ServerError(w, r, "list api tokens", err)
		return
	}
	httpx.JSON(w, http.StatusOK, tokens)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	rawName, _ := fields.String("name") // non-string -> "" -> rejected below
	name, ok := auth.NormalizeTokenName(rawName)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "Nama token wajib diisi (1-60 karakter).")
		return
	}
	rawScope, _ := fields.String("scope")
	// Present but non-string (or null) must be rejected, not defaulted.
	scope, ok := auth.NormalizeTokenScope(rawScope, fields.Has("scope"))
	if !ok {
		httpx.Error(w, http.StatusBadRequest, `Scope token harus "read" atau "write".`)
		return
	}

	gen, err := auth.GenerateAPIToken()
	if err != nil {
		httpx.ServerError(w, r, "generate api token", err)
		return
	}
	rec, err := h.repo.Create(r.Context(), name, gen.Hash, gen.Prefix, scope)
	if errors.Is(err, ErrLimitReached) {
		httpx.Error(w, http.StatusBadRequest,
			fmt.Sprintf("Maksimal %d token aktif. Cabut token lama terlebih dahulu.", auth.MaxActiveTokens))
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "create api token", err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created{Token: rec, Secret: gen.Token})
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathInt(r, "id")
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "ID token tidak valid.")
		return
	}
	revoked, err := h.repo.Revoke(r.Context(), id)
	if err != nil {
		httpx.ServerError(w, r, "revoke api token", err)
		return
	}
	if !revoked {
		httpx.Error(w, http.StatusNotFound, "Token tidak ditemukan atau sudah dicabut.")
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Message{Message: "Token berhasil dicabut."})
}
