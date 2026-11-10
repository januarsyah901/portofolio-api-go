package token

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/januarsyah901/portofolio-api-go/internal/auth"
)

const testSecret = "0123456789abcdef0123456789abcdef"

type fakeRepo struct {
	tokens []Token // insertion order
	hashes []string
	err    error
}

func (f *fakeRepo) List(_ context.Context) ([]Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []Token{}
	for i := len(f.tokens) - 1; i >= 0; i-- {
		out = append(out, f.tokens[i])
	}
	return out, nil
}

func (f *fakeRepo) Create(_ context.Context, name, hash, prefix, scope string) (Token, error) {
	if f.err != nil {
		return Token{}, f.err
	}
	active := 0
	for _, t := range f.tokens {
		if t.RevokedAt == nil {
			active++
		}
	}
	if active >= auth.MaxActiveTokens {
		return Token{}, ErrLimitReached
	}
	t := Token{ID: len(f.tokens) + 1, Name: name, Prefix: prefix, Scope: scope, CreatedAt: time.Now()}
	f.tokens = append(f.tokens, t)
	f.hashes = append(f.hashes, hash)
	return t, nil
}

func (f *fakeRepo) Revoke(_ context.Context, id int) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	for i := range f.tokens {
		if f.tokens[i].ID == id && f.tokens[i].RevokedAt == nil {
			now := time.Now()
			f.tokens[i].RevokedAt = &now
			return true, nil
		}
	}
	return false, nil
}

type env struct {
	h    http.Handler
	repo *fakeRepo
	jwt  string
}

func setup(t *testing.T) *env {
	t.Helper()
	j := auth.NewJWT(testSecret)
	jwt, _ := j.Sign(1)
	repo := &fakeRepo{}
	mux := http.NewServeMux()
	(&Handler{repo: repo}).routes(mux, auth.Guards{JWT: j})
	return &env{h: mux, repo: repo, jwt: jwt}
}

func (e *env) do(method, path, body, authz string) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func (e *env) authed(method, path, body string) (int, string) {
	return e.do(method, path, body, "Bearer "+e.jwt)
}

func msg(body string) string {
	var m map[string]string
	_ = json.Unmarshal([]byte(body), &m)
	return m["message"]
}

func TestAllRoutesRequireJWT(t *testing.T) {
	e := setup(t)
	for _, c := range [][2]string{{"GET", "/api/admin/tokens"}, {"POST", "/api/admin/tokens"}, {"DELETE", "/api/admin/tokens/1"}} {
		if s, b := e.do(c[0], c[1], `{"name":"x"}`, ""); s != 401 || msg(b) != auth.MsgNoToken {
			t.Errorf("%v no auth: %d %s", c, s, b)
		}
		// API tokens can never manage tokens
		if s, _ := e.do(c[0], c[1], `{"name":"x"}`, "Bearer pf_whatever"); s != 401 {
			t.Errorf("%v api token: %d", c, s)
		}
	}
}

func TestCreate(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		msg        string
		scope      string
	}{
		{"missing name", `{}`, 400, "Nama token wajib diisi (1-60 karakter).", ""},
		{"blank name", `{"name":"   "}`, 400, "Nama token wajib diisi (1-60 karakter).", ""},
		{"non-string name", `{"name":5}`, 400, "Nama token wajib diisi (1-60 karakter).", ""},
		{"long name", `{"name":"` + strings.Repeat("a", 61) + `"}`, 400, "Nama token wajib diisi (1-60 karakter).", ""},
		{"empty body", ``, 400, "Nama token wajib diisi (1-60 karakter).", ""},
		{"bad scope", `{"name":"x","scope":"admin"}`, 400, `Scope token harus "read" atau "write".`, ""},
		{"non-string scope", `{"name":"x","scope":1}`, 400, `Scope token harus "read" atau "write".`, ""},
		{"null scope", `{"name":"x","scope":null}`, 400, `Scope token harus "read" atau "write".`, ""},
		{"default scope", `{"name":"  ci  "}`, 201, "", "read"},
		{"write scope", `{"name":"x","scope":"write"}`, 201, "", "write"},
		{"60 chars ok", `{"name":"` + strings.Repeat("a", 60) + `"}`, 201, "", "read"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := setup(t)
			s, b := e.authed("POST", "/api/admin/tokens", c.body)
			if s != c.status {
				t.Fatalf("status %d, want %d: %s", s, c.status, b)
			}
			if c.msg != "" && msg(b) != c.msg {
				t.Errorf("message %q, want %q", msg(b), c.msg)
			}
			if s != 201 {
				return
			}
			var out map[string]any
			_ = json.Unmarshal([]byte(b), &out)
			tok, _ := out["token"].(string)
			if !strings.HasPrefix(tok, "pf_") || out["prefix"] != tok[:8] || out["scope"] != c.scope {
				t.Errorf("body = %s", b)
			}
			if _, leaked := out["tokenHash"]; leaked || strings.Contains(b, auth.HashAPIToken(tok)) {
				t.Error("hash leaked")
			}
			if v, present := out["lastUsedAt"]; !present || v != nil {
				t.Errorf("lastUsedAt must be present and null: %s", b)
			}
			if v, present := out["revokedAt"]; !present || v != nil {
				t.Errorf("revokedAt must be present and null: %s", b)
			}
			if e.repo.hashes[0] != auth.HashAPIToken(tok) {
				t.Error("stored hash does not match returned token")
			}
		})
	}
}

func TestCreateLimit(t *testing.T) {
	e := setup(t)
	for i := 0; i < auth.MaxActiveTokens; i++ {
		if s, b := e.authed("POST", "/api/admin/tokens", `{"name":"t"}`); s != 201 {
			t.Fatalf("token %d: %d %s", i, s, b)
		}
	}
	s, b := e.authed("POST", "/api/admin/tokens", `{"name":"t"}`)
	if s != 400 || msg(b) != "Maksimal 10 token aktif. Cabut token lama terlebih dahulu." {
		t.Fatalf("11th: %d %s", s, b)
	}
	if s, _ := e.authed("DELETE", "/api/admin/tokens/1", ""); s != 200 {
		t.Fatalf("revoke: %d", s)
	}
	if s, _ := e.authed("POST", "/api/admin/tokens", `{"name":"t"}`); s != 201 {
		t.Fatalf("after revoke: %d", s)
	}
}

func TestListNeverLeaksAndOrders(t *testing.T) {
	e := setup(t)
	if s, b := e.authed("GET", "/api/admin/tokens", ""); s != 200 || strings.TrimSpace(b) != "[]" {
		t.Fatalf("empty list: %d %q", s, b)
	}
	e.authed("POST", "/api/admin/tokens", `{"name":"first"}`)
	e.authed("POST", "/api/admin/tokens", `{"name":"second"}`)
	s, b := e.authed("GET", "/api/admin/tokens", "")
	var list []map[string]any
	_ = json.Unmarshal([]byte(b), &list)
	if s != 200 || len(list) != 2 || list[0]["name"] != "second" {
		t.Fatalf("list: %d %s", s, b)
	}
	if strings.Contains(b, "tokenHash") || strings.Contains(b, `"token"`) {
		t.Errorf("secret material in list: %s", b)
	}
	for _, k := range []string{"id", "name", "prefix", "scope", "createdAt", "lastUsedAt", "revokedAt"} {
		if _, ok := list[0][k]; !ok {
			t.Errorf("missing field %s", k)
		}
	}
}

func TestRevoke(t *testing.T) {
	e := setup(t)
	e.authed("POST", "/api/admin/tokens", `{"name":"x"}`)
	cases := []struct {
		name, path string
		status     int
		msg        string
	}{
		{"bad id", "/api/admin/tokens/abc", 400, "ID token tidak valid."},
		{"float id", "/api/admin/tokens/1.5", 400, "ID token tidak valid."},
		{"missing", "/api/admin/tokens/99", 404, "Token tidak ditemukan atau sudah dicabut."},
		{"ok", "/api/admin/tokens/1", 200, "Token berhasil dicabut."},
		{"already revoked", "/api/admin/tokens/1", 404, "Token tidak ditemukan atau sudah dicabut."},
	}
	for _, c := range cases {
		s, b := e.authed("DELETE", c.path, "")
		if s != c.status || msg(b) != c.msg {
			t.Errorf("%s: %d %s", c.name, s, b)
		}
	}
}

func TestServerErrors(t *testing.T) {
	e := setup(t)
	e.repo.err = errors.New("boom")
	for _, c := range [][3]string{{"GET", "/api/admin/tokens", ""}, {"POST", "/api/admin/tokens", `{"name":"x"}`}, {"DELETE", "/api/admin/tokens/1", ""}} {
		s, b := e.authed(c[0], c[1], c[2])
		if s != 500 || msg(b) != "Terjadi kesalahan pada server." {
			t.Errorf("%v: %d %s", c[:2], s, b)
		}
	}
}

func TestTimestampFormat(t *testing.T) {
	ts := time.Date(2026, 11, 10, 16, 36, 28, 123456789, time.FixedZone("x", 7*3600))
	b, _ := json.Marshal(Token{ID: 1, CreatedAt: ts, RevokedAt: &ts})
	if !strings.Contains(string(b), `"createdAt":"2026-11-10T09:36:28.123Z"`) ||
		!strings.Contains(string(b), `"revokedAt":"2026-11-10T09:36:28.123Z"`) ||
		!strings.Contains(string(b), `"lastUsedAt":null`) {
		t.Fatalf("got %s", b)
	}
}
