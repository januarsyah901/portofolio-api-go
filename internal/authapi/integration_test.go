//go:build integration

package authapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/config"
	"github.com/mrfroglab/portofolio-api-go/internal/testutil"
)

func TestIntegrationLoginRoundTrip(t *testing.T) {
	pool := testutil.DB(t)
	j := auth.NewJWT(testSecret)
	g := auth.Guards{JWT: j, Tokens: auth.PGTokenStore{DB: pool}}
	mux := http.NewServeMux()
	Register(mux, app.Deps{Pool: pool, JWT: j, Guards: g,
		Config: config.Config{AdminEmails: []string{"boss@example.com"}}})

	hash, _ := auth.HashPassword("a-long-enough-password")
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO "AdminUser" ("username", "passwordHash") VALUES ('admin', $1)`, hash); err != nil {
		t.Fatal(err)
	}

	do := func(method, path, body, bearer, addr string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = addr
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	if s, _ := do("POST", "/api/auth/login", `{"username":"admin","password":"wrong"}`, "", "1.1.1.1:1"); s != 401 {
		t.Fatalf("wrong password: %d", s)
	}
	if s, _ := do("POST", "/api/auth/login", `{"username":"ghost","password":"x"}`, "", "1.1.1.1:1"); s != 401 {
		t.Fatalf("unknown user: %d", s)
	}
	s, b := do("POST", "/api/auth/login", `{"username":"admin","password":"a-long-enough-password"}`, "", "1.1.1.1:1")
	if s != 200 {
		t.Fatalf("login: %d %v", s, b)
	}
	token := b["token"].(string)
	user := b["user"].(map[string]any)
	if user["username"] != "admin" || user["id"].(float64) != 1 {
		t.Fatalf("user = %v", user)
	}

	s, b = do("GET", "/api/auth/me", "", token, "1.1.1.1:1")
	if s != 200 || b["username"] != "admin" {
		t.Fatalf("me: %d %v", s, b)
	}

	// deleted user -> 404 on /me with a still-valid JWT
	if _, err := pool.Exec(context.Background(), `DELETE FROM "AdminUser"`); err != nil {
		t.Fatal(err)
	}
	if s, _ = do("GET", "/api/auth/me", "", token, "1.1.1.1:1"); s != 404 {
		t.Fatalf("me after delete: %d", s)
	}
}

func TestIntegrationGoogleVerifyCreatesUser(t *testing.T) {
	pool := testutil.DB(t)
	g := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"email":"Boss@Example.com"}`))
	}))
	defer g.Close()
	j := auth.NewJWT(testSecret)
	h := &Handler{repo: pgRepo{db: pool}, jwt: j, admins: []string{"boss@example.com"},
		client: g.Client(), googleURL: g.URL}
	mux := http.NewServeMux()
	h.routes(mux, auth.Guards{JWT: j}, newLimiter())

	for i := 0; i < 2; i++ { // second call reuses the row
		req := httptest.NewRequest("POST", "/api/auth/google-verify", strings.NewReader(`{"accessToken":"good"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("call %d: %d %s", i, rec.Code, rec.Body)
		}
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM "AdminUser" WHERE "username" = 'Boss@Example.com'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("users = %d, %v", n, err)
	}
}
