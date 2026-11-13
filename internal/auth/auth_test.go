package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

const secret = "0123456789abcdef0123456789abcdef"

// Reference values below were produced by the Express API's own libraries
// (node crypto, bcryptjs, jsonwebtoken) so these tests prove cross-compatibility.
const (
	nodeSHA256OfPfTest = "5753113decbb50358b6f07c3615826573aa182fe4c72d150409007df3cf62537"
	nodeBcryptAdmin123 = "$2a$10$B3LsXcE0fIsVEktD2ZCiUub5I4/5m2OtCLgLbTGOuEfRJjhW19qWK"
	// jwt.sign({userId: 1}, secret, {expiresIn: '100y'})
	nodeJWTUser1 = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjEsImlhdCI6MTc5NDMyODE0OCwiZXhwIjo0OTUwMDg4MTQ4fQ.kDx6vorHWbpNitiq5iSAxLbPbYmLxfVS0OB5CWxC9xI"
)

func TestHashMatchesNodeCrypto(t *testing.T) {
	if got := HashAPIToken("pf_test"); got != nodeSHA256OfPfTest {
		t.Fatalf("HashAPIToken = %s, want %s", got, nodeSHA256OfPfTest)
	}
}

func TestVerifiesNodeJWT(t *testing.T) {
	id, err := NewJWT(secret).Verify(nodeJWTUser1)
	if err != nil || id != 1 {
		t.Fatalf("Verify(node token) = %d, %v", id, err)
	}
}

func TestGenerateAPIToken(t *testing.T) {
	g, err := GenerateAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(g.Token, "pf_") || len(g.Token) != 3+43 {
		t.Errorf("token = %q (len %d)", g.Token, len(g.Token))
	}
	if g.Prefix != g.Token[:8] || g.Hash != HashAPIToken(g.Token) {
		t.Errorf("prefix/hash mismatch: %+v", g)
	}
}

func TestExtractBearer(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"Bearer abc":       "abc",
		"bearer abc":       "abc",
		"  Bearer   abc  ": "abc",
		"Basic abc":        "",
		"Bearer":           "",
		"Bearer abc extra": "",
	}
	for in, want := range cases {
		if got := ExtractBearer(in); got != want {
			t.Errorf("ExtractBearer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTokenScopeAndName(t *testing.T) {
	if s, ok := NormalizeTokenScope("", false); !ok || s != "read" {
		t.Error("missing scope should default to read")
	}
	if _, ok := NormalizeTokenScope("admin", true); ok {
		t.Error("unknown scope must be rejected")
	}
	if s, ok := NormalizeTokenScope("write", true); !ok || s != "write" {
		t.Error("write scope must be accepted")
	}
	if n, ok := NormalizeTokenName("  ci  "); !ok || n != "ci" {
		t.Error("name should be trimmed")
	}
	if _, ok := NormalizeTokenName(strings.Repeat("a", 61)); ok {
		t.Error("61 chars must be rejected")
	}
	if _, ok := NormalizeTokenName("   "); ok {
		t.Error("blank name must be rejected")
	}
}

func TestJWTRoundTripAndExpiry(t *testing.T) {
	j := NewJWT(secret)
	tok, err := j.Sign(7)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := j.Verify(tok); err != nil || id != 7 {
		t.Fatalf("Verify = %d, %v", id, err)
	}
	if _, err := NewJWT(strings.Repeat("z", 32)).Verify(tok); err == nil {
		t.Error("token signed with another secret must fail")
	}
	j.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if _, err := j.Verify(tok); err == nil {
		t.Error("expired token must fail")
	}
}

func TestPasswordHashing(t *testing.T) {
	h, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "s3cret-password") || CheckPassword(h, "wrong") {
		t.Error("bcrypt round trip failed")
	}
	if CheckPassword("", "anything") {
		t.Error("unknown user must never match")
	}
	if !CheckPassword(nodeBcryptAdmin123, "admin123") {
		t.Error("hash produced by bcryptjs must verify")
	}
}

type fakeStore struct {
	mu      sync.Mutex
	tokens  map[string]APIToken
	touched []int
}

func (f *fakeStore) FindByHash(_ context.Context, hash string) (APIToken, error) {
	t, ok := f.tokens[hash]
	if !ok {
		return APIToken{}, db.ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) TouchLastUsed(_ context.Context, id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.touched = append(f.touched, id)
	return nil
}

func TestGuards(t *testing.T) {
	j := NewJWT(secret)
	jwtTok, _ := j.Sign(1)
	revoked := time.Now()
	store := &fakeStore{tokens: map[string]APIToken{
		HashAPIToken("pf_read"):    {ID: 1, Scope: "read"},
		HashAPIToken("pf_write"):   {ID: 2, Scope: "write"},
		HashAPIToken("pf_revoked"): {ID: 3, Scope: "write", RevokedAt: &revoked},
	}}
	g := Guards{JWT: j, Tokens: store}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })

	type tc struct {
		name, guard, method, auth string
		status                    int
		msg                       string
	}
	cases := []tc{
		{"jwt only, no header", "jwt", "GET", "", 401, MsgNoToken},
		{"jwt only, garbage", "jwt", "GET", "Bearer nope", 401, MsgBadJWT},
		{"jwt only, api token rejected", "jwt", "GET", "Bearer pf_write", 401, MsgBadJWT},
		{"jwt only, valid", "jwt", "GET", "Bearer " + jwtTok, 418, ""},
		{"read, jwt", "read", "GET", "Bearer " + jwtTok, 418, ""},
		{"read, read token GET", "read", "GET", "Bearer pf_read", 418, ""},
		{"read, read token POST", "read", "POST", "Bearer pf_read", 403, MsgReadOnlyToken},
		{"read, unknown token", "read", "GET", "Bearer pf_unknown", 401, MsgBadAPIToken},
		{"read, revoked token", "read", "GET", "Bearer pf_revoked", 401, MsgBadAPIToken},
		{"write, read token", "write", "POST", "Bearer pf_read", 403, MsgNeedWriteScope},
		{"write, write token", "write", "DELETE", "Bearer pf_write", 418, ""},
		{"write, no header", "write", "POST", "", 401, MsgNoToken},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var h http.Handler
			switch c.guard {
			case "jwt":
				h = g.JWTOnly(ok)
			case "read":
				h = g.AdminOrRead(ok)
			default:
				h = g.AdminOrWrite(ok)
			}
			req := httptest.NewRequest(c.method, "/", nil)
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body)
			}
			if c.msg != "" {
				var body map[string]string
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body["message"] != c.msg {
					t.Errorf("message = %q, want %q", body["message"], c.msg)
				}
			}
		})
	}
}
