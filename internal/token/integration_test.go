//go:build integration

package token

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/testutil"
)

func TestIntegrationTokenLifecycle(t *testing.T) {
	pool := testutil.DB(t)
	j := auth.NewJWT(testSecret)
	guards := auth.Guards{JWT: j, Tokens: auth.PGTokenStore{DB: pool}}
	adminJWT, _ := j.Sign(1)

	mux := http.NewServeMux()
	Register(mux, appDeps(pool, j, guards))
	// a GET endpoint guarded by AdminOrRead, standing in for any "admin-list" route
	mux.Handle("GET /probe", guards.AdminOrRead(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	do := func(method, path, body, bearer string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	// create
	s, b := do("POST", "/api/admin/tokens", `{"name":"ci"}`, adminJWT)
	if s != 201 {
		t.Fatalf("create: %d %s", s, b)
	}
	secret := between(b, `"token":"`, `"`)
	if !strings.HasPrefix(secret, "pf_") {
		t.Fatalf("no token in %s", b)
	}

	// list shows it, without secret
	s, b = do("GET", "/api/admin/tokens", "", adminJWT)
	if s != 200 || !strings.Contains(b, `"name":"ci"`) || strings.Contains(b, secret) || strings.Contains(b, "tokenHash") {
		t.Fatalf("list: %d %s", s, b)
	}

	// token works on AdminOrRead, but cannot manage tokens
	if s, _ = do("GET", "/probe", "", secret); s != http.StatusNoContent {
		t.Fatalf("token on read guard: %d", s)
	}
	if s, _ = do("GET", "/api/admin/tokens", "", secret); s != 401 {
		t.Fatalf("token on jwt-only route: %d", s)
	}

	// lastUsedAt is updated asynchronously
	deadline := time.Now().Add(3 * time.Second)
	for {
		var used bool
		if err := pool.QueryRow(context.Background(),
			`SELECT "lastUsedAt" IS NOT NULL FROM "ApiToken" LIMIT 1`).Scan(&used); err != nil {
			t.Fatal(err)
		}
		if used {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lastUsedAt never set")
		}
		time.Sleep(25 * time.Millisecond)
	}

	// revoke -> rejected; second revoke -> 404
	if s, b = do("DELETE", "/api/admin/tokens/1", "", adminJWT); s != 200 {
		t.Fatalf("revoke: %d %s", s, b)
	}
	if s, _ = do("GET", "/probe", "", secret); s != 401 {
		t.Fatalf("revoked token accepted: %d", s)
	}
	if s, _ = do("DELETE", "/api/admin/tokens/1", "", adminJWT); s != 404 {
		t.Fatalf("double revoke: %d", s)
	}
	if s, _ = do("DELETE", "/api/admin/tokens/99999999999", "", adminJWT); s != 404 {
		t.Fatalf("out-of-range id: %d", s)
	}
}

func TestIntegrationTokenLimitIsRaceSafe(t *testing.T) {
	pool := testutil.DB(t)
	j := auth.NewJWT(testSecret)
	adminJWT, _ := j.Sign(1)
	mux := http.NewServeMux()
	Register(mux, appDeps(pool, j, auth.Guards{JWT: j}))

	post := func() int {
		req := httptest.NewRequest("POST", "/api/admin/tokens", strings.NewReader(`{"name":"x"}`))
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	// 11th sequential create is rejected
	for i := 0; i < auth.MaxActiveTokens; i++ {
		if c := post(); c != 201 {
			t.Fatalf("token %d: %d", i+1, c)
		}
	}
	if c := post(); c != 400 {
		t.Fatalf("11th token: %d, want 400", c)
	}

	// revoke everything, then hammer concurrently: exactly 10 may win
	if _, err := pool.Exec(context.Background(), `UPDATE "ApiToken" SET "revokedAt" = now()`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := post()
			mu.Lock()
			codes[c]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[201] != auth.MaxActiveTokens || codes[400] != 15 {
		t.Fatalf("concurrent creates: %v (want 10x201, 15x400)", codes)
	}
}

func between(s, a, b string) string {
	i := strings.Index(s, a)
	if i < 0 {
		return ""
	}
	s = s[i+len(a):]
	return s[:strings.Index(s, b)]
}
