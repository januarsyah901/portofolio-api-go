package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/januarsyah901/portofolio-api-go/internal/db"
	"github.com/januarsyah901/portofolio-api-go/internal/httpx"
)

// Messages identical to src/middleware/auth.ts.
const (
	MsgNoToken        = "Akses ditolak. Token tidak ditemukan."
	MsgBadJWT         = "Token tidak valid atau kedaluwarsa."
	MsgReadOnlyToken  = "Token API hanya dapat digunakan untuk akses baca (read-only)."
	MsgBadAPIToken    = "Token API tidak valid atau sudah dicabut."
	MsgNeedWriteScope = "Token API ini hanya memiliki scope read. Buat token dengan scope write untuk mengubah data."
)

// APIToken is the stored record the guards need.
type APIToken struct {
	ID        int
	Scope     string
	RevokedAt *time.Time
}

// TokenStore looks up API tokens by hash and records their use.
type TokenStore interface {
	FindByHash(ctx context.Context, hash string) (APIToken, error) // db.ErrNotFound when missing
	TouchLastUsed(ctx context.Context, id int) error
}

// Guards builds the middleware that protects admin routes.
type Guards struct {
	JWT    *JWT
	Tokens TokenStore
}

type userIDKey struct{}

// UserID returns the admin user id set by a successful JWT check (0 for API tokens).
func UserID(ctx context.Context) int {
	id, _ := ctx.Value(userIDKey{}).(int)
	return id
}

// JWTOnly accepts only an admin JWT. Used for /auth/me, token management and uploads.
func (g Guards) JWTOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Express took the second space-separated part of the header without checking the scheme.
		token := secondPart(r.Header.Get("Authorization"))
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, MsgNoToken)
			return
		}
		userID, err := g.JWT.Verify(token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, MsgBadJWT)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, userID)))
	})
}

// AdminOrRead accepts an admin JWT, or any active API token on GET/HEAD.
func (g Guards) AdminOrRead(next http.Handler) http.Handler { return g.adminOrToken(false, next) }

// AdminOrWrite accepts an admin JWT, or an active API token with scope "write".
func (g Guards) AdminOrWrite(next http.Handler) http.Handler { return g.adminOrToken(true, next) }

func (g Guards) adminOrToken(write bool, next http.Handler) http.Handler {
	jwtOnly := g.JWTOnly(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential := ExtractBearer(r.Header.Get("Authorization"))
		if !IsAPIToken(credential) {
			jwtOnly.ServeHTTP(w, r)
			return
		}
		if !write && r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpx.Error(w, http.StatusForbidden, MsgReadOnlyToken)
			return
		}
		record, err := g.Tokens.FindByHash(r.Context(), HashAPIToken(credential))
		if err != nil && !isNotFound(err) {
			httpx.ServerError(w, r, "verify api token", err)
			return
		}
		if err != nil || record.RevokedAt != nil {
			httpx.Error(w, http.StatusUnauthorized, MsgBadAPIToken)
			return
		}
		if write && !CanWrite(record.Scope) {
			httpx.Error(w, http.StatusForbidden, MsgNeedWriteScope)
			return
		}
		// Fire-and-forget like the Express version; a failure is only logged.
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := g.Tokens.TouchLastUsed(ctx, id); err != nil {
				slog.Error("update token lastUsedAt", "err", err, "token_id", id)
			}
		}(record.ID)
		next.ServeHTTP(w, r)
	})
}

// secondPart mirrors `header.split(' ')[1]`.
func secondPart(header string) string {
	parts := strings.Split(header, " ")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func isNotFound(err error) bool { return errors.Is(err, db.ErrNotFound) }
