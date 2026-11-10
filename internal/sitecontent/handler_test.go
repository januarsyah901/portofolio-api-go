package sitecontent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
	"github.com/januarsyah901/portofolio-api-go/internal/auth"
)

type fakeRepo struct {
	mu   sync.Mutex
	rows map[string]json.RawMessage
	err  error
}

func (f *fakeRepo) List(context.Context) (map[string]json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]json.RawMessage{}
	for k, v := range f.rows {
		out[k] = v
	}
	return out, nil
}

func (f *fakeRepo) Upsert(_ context.Context, key string, value []byte) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.rows[key] = json.RawMessage(value)
	return f.rows[key], nil
}

func (f *fakeRepo) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	delete(f.rows, key)
	return nil
}

func setup(t *testing.T) (*http.ServeMux, *fakeRepo, string) {
	t.Helper()
	j := auth.NewJWT("secret-for-tests-secret-for-tests")
	tok, err := j.Sign(1)
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeRepo{rows: map[string]json.RawMessage{}}
	mux := http.NewServeMux()
	register(mux, app.Deps{Guards: auth.Guards{JWT: j}}, repo)
	return mux, repo, tok
}

func do(mux http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func msg(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var m struct{ Message string }
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return m.Message
}

func TestGetEmptyAndPopulated(t *testing.T) {
	mux, repo, _ := setup(t)
	rec := do(mux, "GET", "/api/site-content", "", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "{}" {
		t.Fatalf("empty: %d %s", rec.Code, rec.Body)
	}
	repo.rows["about"] = json.RawMessage(`{"name":"x"}`)
	rec = do(mux, "GET", "/api/site-content/", "", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"about":{"name":"x"}}` {
		t.Fatalf("populated: %d %s", rec.Code, rec.Body)
	}
}

func TestPutRequiresAuth(t *testing.T) {
	mux, _, _ := setup(t)
	if rec := do(mux, "PUT", "/api/site-content/about", "", `{}`); rec.Code != 401 {
		t.Errorf("put: %d", rec.Code)
	}
	if rec := do(mux, "DELETE", "/api/site-content/about", "", ""); rec.Code != 401 {
		t.Errorf("delete: %d", rec.Code)
	}
}

func TestPutUnknownKey(t *testing.T) {
	mux, _, tok := setup(t)
	rec := do(mux, "PUT", "/api/site-content/hero", tok, `{}`)
	want := `Section "hero" tidak dikenal. Pilihan: experience, skills, about, contact.`
	if rec.Code != 404 || msg(t, rec) != want {
		t.Errorf("%d %q", rec.Code, msg(t, rec))
	}
	rec = do(mux, "DELETE", "/api/site-content/hero", tok, "")
	if rec.Code != 404 || msg(t, rec) != want {
		t.Errorf("delete %d %q", rec.Code, msg(t, rec))
	}
}

func TestPutBodyShapes(t *testing.T) {
	mux, _, tok := setup(t)
	for _, body := range []string{`[]`, `"str"`, `null`, `5`, `[1,2]`} {
		rec := do(mux, "PUT", "/api/site-content/experience", tok, body)
		if rec.Code != 400 || msg(t, rec) != `"experience" harus berupa objek.` {
			t.Errorf("%s: %d %q", body, rec.Code, msg(t, rec))
		}
	}
	rec := do(mux, "PUT", "/api/site-content/experience", tok, ``)
	if rec.Code != 400 || msg(t, rec) != `"eyebrow" harus berupa teks.` {
		t.Errorf("empty: %d %q", rec.Code, msg(t, rec))
	}
	rec = do(mux, "PUT", "/api/site-content/experience", tok, `{bad`)
	if rec.Code != 400 || msg(t, rec) != "Body request harus berupa JSON yang valid." {
		t.Errorf("bad json: %d %q", rec.Code, msg(t, rec))
	}
	rec = do(mux, "PUT", "/api/site-content/experience", tok, `{"eyebrow":"`+strings.Repeat("a", 2<<20)+`"}`)
	if rec.Code != 413 {
		t.Errorf("large: %d", rec.Code)
	}
}

func TestPutValidationError(t *testing.T) {
	mux, repo, tok := setup(t)
	rec := do(mux, "PUT", "/api/site-content/experience", tok,
		`{"eyebrow":"a","title":"b","items":[{"period":"p","badge":"b","title":" ","role":"r","description":"d"}]}`)
	if rec.Code != 400 || msg(t, rec) != `"items[0].title" tidak boleh kosong.` {
		t.Errorf("%d %q", rec.Code, msg(t, rec))
	}
	if len(repo.rows) != 0 {
		t.Error("nothing should be stored")
	}
}

func TestPutGetDeleteRoundTrip(t *testing.T) {
	mux, repo, tok := setup(t)
	body := `{"ctaTitle":" Hi ","ctaSubtitle":"s","brandDesc":"b","email":"a@b.co","location":"l",
	  "github":"https://g.co/x","linkedin":"https://l.co/x","x":"https://x.co/x","instagram":"https://i.co/x","evil":1}`
	rec := do(mux, "PUT", "/api/site-content/contact", tok, body)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Key   string
		Value map[string]any
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Key != "contact" || resp.Value["ctaTitle"] != "Hi" || resp.Value["evil"] != nil {
		t.Errorf("resp: %s", rec.Body)
	}
	if _, ok := repo.rows["contact"]; !ok {
		t.Error("not stored")
	}
	rec = do(mux, "GET", "/api/site-content", "", "")
	if !strings.Contains(rec.Body.String(), `"contact":{"ctaTitle":"Hi"`) {
		t.Errorf("get: %s", rec.Body)
	}
	for i := 0; i < 2; i++ { // second delete: row already gone, same message
		rec = do(mux, "DELETE", "/api/site-content/contact", tok, "")
		if rec.Code != 200 || msg(t, rec) != `Section "contact" dihapus. Website memakai teks bawaan.` {
			t.Errorf("delete %d: %d %q", i, rec.Code, msg(t, rec))
		}
	}
	if len(repo.rows) != 0 {
		t.Error("not deleted")
	}
}

func TestRepoErrors(t *testing.T) {
	mux, repo, tok := setup(t)
	repo.err = context.DeadlineExceeded
	valid := `{"eyebrow":"a","name":"n","paragraphs":["p"],"summaryTitle":"s","summary":[]}`
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/site-content", ""},
		{"PUT", "/api/site-content/about", valid},
		{"DELETE", "/api/site-content/about", ""},
	} {
		rec := do(mux, c.method, c.path, tok, c.body)
		if rec.Code != 500 || msg(t, rec) != "Terjadi kesalahan pada server." {
			t.Errorf("%s: %d %q", c.method, rec.Code, msg(t, rec))
		}
	}
}
