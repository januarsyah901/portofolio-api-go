package authapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/db"
	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

const (
	googleUserinfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	googleTimeout     = 5 * time.Second
)

// Handler serves /api/auth/*.
type Handler struct {
	repo      Repo
	jwt       *auth.JWT
	admins    []string // lowercase allowlist (PRD S3)
	client    *http.Client
	googleURL string
}

type authResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

func (h *Handler) issue(w http.ResponseWriter, r *http.Request, u User) {
	token, err := h.jwt.Sign(u.ID)
	if err != nil {
		httpx.ServerError(w, r, "sign jwt", err)
		return
	}
	httpx.JSON(w, http.StatusOK, authResponse{Token: token, User: u})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	if !fields.Truthy("username") || !fields.Truthy("password") {
		httpx.Error(w, http.StatusBadRequest, "Username dan password wajib diisi.")
		return
	}
	// Non-string values can never match a user/password.
	username, _ := fields.String("username")
	password, _ := fields.String("password")

	user, hash, err := h.repo.FindByUsername(r.Context(), username)
	found := err == nil
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		httpx.ServerError(w, r, "login lookup", err)
		return
	}
	if !found {
		hash = "" // still burns a bcrypt compare (PRD S10)
	}
	if !auth.CheckPassword(hash, password) || !found {
		httpx.Error(w, http.StatusUnauthorized, "Username atau password salah.")
		return
	}
	h.issue(w, r, user)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.repo.FindByID(r.Context(), auth.UserID(r.Context()))
	if errors.Is(err, db.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "User tidak ditemukan.")
		return
	}
	if err != nil {
		httpx.ServerError(w, r, "fetch current user", err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handler) googleVerify(w http.ResponseWriter, r *http.Request) {
	fields, ok := httpx.DecodeObject(w, r)
	if !ok {
		return
	}
	if !fields.Truthy("accessToken") {
		httpx.Error(w, http.StatusBadRequest, "Access token Google wajib disertakan.")
		return
	}
	accessToken, isString := fields.String("accessToken")
	if !isString {
		httpx.Error(w, http.StatusUnauthorized, "Token Google tidak valid atau kedaluwarsa.")
		return
	}

	email, status, err := h.googleEmail(r.Context(), accessToken)
	if err != nil {
		httpx.ServerError(w, r, "google userinfo", err)
		return
	}
	if status != 0 {
		httpx.Error(w, status, "Token Google tidak valid atau kedaluwarsa.")
		return
	}
	if email == "" || !h.allowed(email) {
		httpx.Error(w, http.StatusForbidden,
			fmt.Sprintf("Akses ditolak. Email %s tidak memiliki izin admin.", email))
		return
	}

	user, _, err := h.repo.FindByUsername(r.Context(), email)
	if errors.Is(err, db.ErrNotFound) {
		user, err = h.createGoogleUser(r.Context(), email)
	}
	if err != nil {
		httpx.ServerError(w, r, "google-verify user", err)
		return
	}
	h.issue(w, r, user)
}

func (h *Handler) allowed(email string) bool {
	e := strings.ToLower(email)
	for _, a := range h.admins {
		if a == e {
			return true
		}
	}
	return false
}

// createGoogleUser inserts a user with an unguessable random password hash. A
// concurrent first login for the same email is resolved by re-reading the row.
func (h *Handler) createGoogleUser(ctx context.Context, email string) (User, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return User{}, err
	}
	hash, err := auth.HashPassword(base64.RawURLEncoding.EncodeToString(b))
	if err != nil {
		return User{}, err
	}
	u, err := h.repo.Create(ctx, email, hash)
	if db.IsUniqueViolation(err) {
		u, _, err = h.repo.FindByUsername(ctx, email)
	}
	return u, err
}

// googleEmail asks Google who owns accessToken. A non-zero status means the
// token was rejected (respond with that status); err means an infrastructure failure.
func (h *Handler) googleEmail(ctx context.Context, accessToken string) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, googleTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.googleURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken) // PRD S4: never in the URL
	resp, err := h.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", http.StatusUnauthorized, nil
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return "", 0, err
	}
	return info.Email, 0, nil
}
