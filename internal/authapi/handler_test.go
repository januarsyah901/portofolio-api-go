package authapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

const testSecret = "0123456789abcdef0123456789abcdef"

type fakeRepo struct {
	users  map[string]User
	hashes map[string]string
	nextID int
	err    error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[string]User{}, hashes: map[string]string{}, nextID: 1}
}

func (f *fakeRepo) add(username, password string) User {
	h, _ := auth.HashPassword(password)
	u := User{ID: f.nextID, Username: username}
	f.nextID++
	f.users[username], f.hashes[username] = u, h
	return u
}

func (f *fakeRepo) FindByUsername(_ context.Context, username string) (User, string, error) {
	if f.err != nil {
		return User{}, "", f.err
	}
	u, ok := f.users[username]
	if !ok {
		return User{}, "", db.ErrNotFound
	}
	return u, f.hashes[username], nil
}

func (f *fakeRepo) FindByID(_ context.Context, id int) (User, error) {
	if f.err != nil {
		return User{}, f.err
	}
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, db.ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, username, hash string) (User, error) {
	if _, dup := f.users[username]; dup {
		return User{}, &pgconn.PgError{Code: "23505"}
	}
	u := User{ID: f.nextID, Username: username}
	f.nextID++
	f.users[username], f.hashes[username] = u, hash
	return u, nil
}

type env struct {
	h    http.Handler
	repo *fakeRepo
	jwt  *auth.JWT
	hand *Handler
	addr string // fixed client address; unique per request when empty
	n    int
}

func setup(t *testing.T, googleURL string) *env {
	t.Helper()
	repo := newFakeRepo()
	j := auth.NewJWT(testSecret)
	h := &Handler{repo: repo, jwt: j, admins: []string{"admin@example.com"},
		client: &http.Client{}, googleURL: googleURL}
	mux := http.NewServeMux()
	h.routes(mux, auth.Guards{JWT: j}, newLimiter())
	return &env{h: mux, repo: repo, jwt: j, hand: h}
}

func (e *env) do(method, path, body, authz string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	e.n++
	req.RemoteAddr = fmt.Sprintf("10.0.%d.%d:1234", e.n/250, e.n%250)
	if e.addr != "" {
		req.RemoteAddr = e.addr
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestLogin(t *testing.T) {
	e := setup(t, "")
	u := e.repo.add("admin", "correct-horse-battery")
	cases := []struct {
		name, body string
		status     int
		msg        string
	}{
		{"empty body", ``, 400, "Username dan password wajib diisi."},
		{"missing password", `{"username":"admin"}`, 400, "Username dan password wajib diisi."},
		{"empty username", `{"username":"","password":"x"}`, 400, "Username dan password wajib diisi."},
		{"non-object", `[1]`, 400, "Username dan password wajib diisi."},
		{"bad json", `{`, 400, httpxInvalidJSON},
		{"unknown user", `{"username":"nobody","password":"x"}`, 401, "Username atau password salah."},
		{"wrong password", `{"username":"admin","password":"nope"}`, 401, "Username atau password salah."},
		{"non-string password", `{"username":"admin","password":123}`, 401, "Username atau password salah."},
		{"ok", `{"username":"admin","password":"correct-horse-battery"}`, 200, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.do("POST", "/api/auth/login", c.body, "")
			if status != c.status {
				t.Fatalf("status = %d, want %d (%v)", status, c.status, body)
			}
			if c.msg != "" && body["message"] != c.msg {
				t.Errorf("message = %v, want %q", body["message"], c.msg)
			}
			if c.status == 200 {
				user := body["user"].(map[string]any)
				if int(user["id"].(float64)) != u.ID || user["username"] != "admin" {
					t.Errorf("user = %v", user)
				}
				id, err := e.jwt.Verify(body["token"].(string))
				if err != nil || id != u.ID {
					t.Errorf("token verify = %d, %v", id, err)
				}
			}
		})
	}
}

const httpxInvalidJSON = "Body request harus berupa JSON yang valid."

func TestLoginServerError(t *testing.T) {
	e := setup(t, "")
	e.repo.err = context.DeadlineExceeded
	status, body := e.do("POST", "/api/auth/login", `{"username":"a","password":"b"}`, "")
	if status != 500 || body["message"] != "Terjadi kesalahan pada server." {
		t.Fatalf("got %d %v", status, body)
	}
}

func TestMe(t *testing.T) {
	e := setup(t, "")
	u := e.repo.add("admin", "pw")
	tok, _ := e.jwt.Sign(u.ID)
	ghost, _ := e.jwt.Sign(999)

	if s, b := e.do("GET", "/api/auth/me", "", ""); s != 401 || b["message"] != auth.MsgNoToken {
		t.Errorf("no token: %d %v", s, b)
	}
	if s, _ := e.do("GET", "/api/auth/me", "", "Bearer junk"); s != 401 {
		t.Errorf("bad token: %d", s)
	}
	if s, b := e.do("GET", "/api/auth/me", "", "Bearer "+ghost); s != 404 || b["message"] != "User tidak ditemukan." {
		t.Errorf("missing user: %d %v", s, b)
	}
	s, b := e.do("GET", "/api/auth/me", "", "Bearer "+tok)
	if s != 200 || b["username"] != "admin" || int(b["id"].(float64)) != u.ID || len(b) != 2 {
		t.Errorf("me: %d %v", s, b)
	}
	e.repo.err = context.Canceled
	if s, _ := e.do("GET", "/api/auth/me", "", "Bearer "+tok); s != 500 {
		t.Errorf("repo error: %d", s)
	}
}

func TestGoogleVerify(t *testing.T) {
	var gotAuth, gotQuery atomic.Value
	email := "Admin@Example.com"
	status := http.StatusOK
	g := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		gotQuery.Store(r.URL.RawQuery)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"email": email})
	}))
	defer g.Close()
	e := setup(t, g.URL)

	if s, b := e.do("POST", "/api/auth/google-verify", `{}`, ""); s != 400 || b["message"] != "Access token Google wajib disertakan." {
		t.Errorf("missing token: %d %v", s, b)
	}

	// success: user auto-created, token sent in header only
	s, b := e.do("POST", "/api/auth/google-verify", `{"accessToken":"tok123"}`, "")
	if s != 200 {
		t.Fatalf("ok path: %d %v", s, b)
	}
	if gotAuth.Load() != "Bearer tok123" || gotQuery.Load() != "" {
		t.Errorf("auth header %v query %q", gotAuth.Load(), gotQuery.Load())
	}
	if _, ok := e.repo.users["Admin@Example.com"]; !ok {
		t.Error("user not created under the Google email")
	}
	first := b["user"].(map[string]any)["id"]
	// second login reuses the row
	s, b = e.do("POST", "/api/auth/google-verify", `{"accessToken":"tok123"}`, "")
	if s != 200 || b["user"].(map[string]any)["id"] != first || len(e.repo.users) != 1 {
		t.Errorf("second login: %d %v", s, b)
	}

	// not on allowlist
	email = "evil@example.com"
	s, b = e.do("POST", "/api/auth/google-verify", `{"accessToken":"t"}`, "")
	if s != 403 || b["message"] != "Akses ditolak. Email evil@example.com tidak memiliki izin admin." {
		t.Errorf("forbidden: %d %v", s, b)
	}
	// no email at all
	email = ""
	s, b = e.do("POST", "/api/auth/google-verify", `{"accessToken":"t"}`, "")
	if s != 403 || b["message"] != "Akses ditolak. Email  tidak memiliki izin admin." {
		t.Errorf("empty email: %d %v", s, b)
	}
	// Google rejects
	status = http.StatusUnauthorized
	s, b = e.do("POST", "/api/auth/google-verify", `{"accessToken":"t"}`, "")
	if s != 401 || b["message"] != "Token Google tidak valid atau kedaluwarsa." {
		t.Errorf("google 401: %d %v", s, b)
	}
}

func TestGoogleVerifyUnreachable(t *testing.T) {
	g := httptest.NewServer(http.NotFoundHandler())
	url := g.URL
	g.Close()
	e := setup(t, url)
	if s, _ := e.do("POST", "/api/auth/google-verify", `{"accessToken":"t"}`, ""); s != 500 {
		t.Errorf("unreachable google: %d, want 500", s)
	}
}

func TestRateLimitOnLoginAndGoogleOnly(t *testing.T) {
	e := setup(t, "http://127.0.0.1:1")
	e.addr = "10.9.9.9:4000"
	tok, _ := e.jwt.Sign(1)
	e.repo.add("a", "b")
	for i := 0; i < 5; i++ {
		if s, _ := e.do("POST", "/api/auth/login", `{"username":"a","password":"x"}`, ""); s != 401 {
			t.Fatalf("attempt %d: %d", i, s)
		}
	}
	s, b := e.do("POST", "/api/auth/login", `{"username":"a","password":"x"}`, "")
	if s != 429 || b["message"] != "Terlalu banyak percobaan. Coba lagi nanti." {
		t.Fatalf("6th: %d %v", s, b)
	}
	// shared bucket across login + google-verify
	if s, _ := e.do("POST", "/api/auth/google-verify", `{"accessToken":"t"}`, ""); s != 429 {
		t.Errorf("google-verify should share the bucket: %d", s)
	}
	// /me is not rate limited
	for i := 0; i < 10; i++ {
		if s, _ := e.do("GET", "/api/auth/me", "", "Bearer "+tok); s == 429 {
			t.Fatal("/me must not be rate limited")
		}
	}
}
