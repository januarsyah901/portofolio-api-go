package blog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/db"
)

type fakeRepo struct {
	items  []Video
	nextID int
	err    error
}

func (f *fakeRepo) List(_ context.Context, onlyVisible bool) ([]Video, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []Video{}
	for _, it := range f.items {
		if !onlyVisible || it.Visible {
			out = append(out, it)
		}
	}
	return out, nil
}

func (f *fakeRepo) idx(id int) int {
	for i, it := range f.items {
		if it.ID == id {
			return i
		}
	}
	return -1
}

func (f *fakeRepo) Get(_ context.Context, id int) (Video, error) {
	if f.err != nil {
		return Video{}, f.err
	}
	if i := f.idx(id); i >= 0 {
		return f.items[i], nil
	}
	return Video{}, db.ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, in Video) (Video, error) {
	if f.err != nil {
		return Video{}, f.err
	}
	f.nextID++
	in.ID, in.CreatedAt = f.nextID, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	f.items = append(f.items, in)
	return in, nil
}

func (f *fakeRepo) Update(_ context.Context, id int, in Video) (Video, error) {
	i := f.idx(id)
	if i < 0 {
		return Video{}, db.ErrNotFound
	}
	in.ID, in.CreatedAt = id, f.items[i].CreatedAt
	f.items[i] = in
	return in, nil
}

func (f *fakeRepo) Delete(_ context.Context, id int) error {
	i := f.idx(id)
	if i < 0 {
		return db.ErrNotFound
	}
	f.items = append(f.items[:i], f.items[i+1:]...)
	return nil
}

const testSecret = "0123456789abcdef0123456789abcdef"

func setup(t *testing.T, repo *fakeRepo) (http.Handler, string) {
	t.Helper()
	jwt := auth.NewJWT(testSecret)
	tok, err := jwt.Sign(1)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerWith(mux, app.Deps{JWT: jwt, Guards: auth.Guards{JWT: jwt}}, repo)
	return mux, tok
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func msgOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var m struct{ Message string }
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return m.Message
}

func TestPublicListOnlyVisibleAndEmptyIsArray(t *testing.T) {
	repo := &fakeRepo{}
	h, tok := setup(t, repo)
	if rec := do(h, "GET", "/api/blog", "", ""); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %q", rec.Code, rec.Body.String())
	}
	repo.items = []Video{{ID: 1, VideoID: "a", Visible: true}, {ID: 2, VideoID: "b"}}
	var got []Video
	rec := do(h, "GET", "/api/blog", "", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("public list = %+v", got)
	}
	rec = do(h, "GET", "/api/blog/admin-list", tok, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Fatalf("admin list = %+v", got)
	}
}

func TestAuthRequired(t *testing.T) {
	h, _ := setup(t, &fakeRepo{})
	for _, c := range [][2]string{{"GET", "/api/blog/admin-list"}, {"POST", "/api/blog"}, {"PUT", "/api/blog/1"}, {"DELETE", "/api/blog/1"}} {
		if rec := do(h, c[0], c[1], "", "{}"); rec.Code != 401 {
			t.Errorf("%s %s = %d, want 401", c[0], c[1], rec.Code)
		}
	}
}

func TestCreate(t *testing.T) {
	h, tok := setup(t, &fakeRepo{})
	rec := do(h, "POST", "/api/blog", tok, `{"videoId":"abc"}`)
	if rec.Code != 201 {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body)
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"id", "videoId", "caption", "tags", "order", "visible", "createdAt"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q in %v", k, raw)
		}
	}
	if raw["caption"] != "" || raw["order"] != float64(0) || raw["visible"] != true || len(raw["tags"].([]any)) != 0 {
		t.Errorf("defaults wrong: %v", raw)
	}

	rec = do(h, "POST", "/api/blog", tok, `{}`)
	if rec.Code != 400 || msgOf(t, rec) != MsgRequired {
		t.Fatalf("missing fields: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, "POST", "/api/blog", tok, `{"videoId":123}`)
	if rec.Code != 400 {
		t.Fatalf("non-string name = %d", rec.Code)
	}
	if rec = do(h, "POST", "/api/blog", tok, `{bad`); rec.Code != 400 {
		t.Fatalf("bad json = %d", rec.Code)
	}
}

func TestUpdate(t *testing.T) {
	repo := &fakeRepo{items: []Video{{ID: 1, VideoID: "A", Tags: []string{}, Visible: true}}}
	h, tok := setup(t, repo)

	rec := do(h, "PUT", "/api/blog/1", tok, `{"videoId":"Z","visible":false}`)
	if rec.Code != 200 || repo.items[0].VideoID != "Z" || repo.items[0].Visible {
		t.Fatalf("update: %d %s %+v", rec.Code, rec.Body, repo.items[0])
	}
	if rec = do(h, "PUT", "/api/blog/abc", tok, `{}`); rec.Code != 400 || msgOf(t, rec) != MsgBadID {
		t.Fatalf("bad id: %d %s", rec.Code, rec.Body)
	}
	if rec = do(h, "PUT", "/api/blog/99", tok, `{}`); rec.Code != 404 || msgOf(t, rec) != MsgNotFound {
		t.Fatalf("missing: %d %s", rec.Code, rec.Body)
	}
	if rec = do(h, "PUT", "/api/blog/99999999999", tok, `{}`); rec.Code != 404 {
		t.Fatalf("huge id: %d", rec.Code)
	}
	if rec = do(h, "PUT", "/api/blog/1", tok, `{"videoId":null}`); rec.Code != 400 {
		t.Fatalf("null name: %d", rec.Code)
	}
	if rec = do(h, "PUT", "/api/blog/1", tok, `{"company":null}`); rec.Code != 200 {
		t.Fatalf("caption: %d", rec.Code)
	}
}

func TestDelete(t *testing.T) {
	repo := &fakeRepo{items: []Video{{ID: 1}}}
	h, tok := setup(t, repo)
	if rec := do(h, "DELETE", "/api/blog/x", tok, ""); rec.Code != 400 || msgOf(t, rec) != MsgBadID {
		t.Fatalf("bad id: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "DELETE", "/api/blog/1", tok, ""); rec.Code != 200 || msgOf(t, rec) != MsgDeleted {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "DELETE", "/api/blog/1", tok, ""); rec.Code != 404 || msgOf(t, rec) != MsgNotFound {
		t.Fatalf("again: %d %s", rec.Code, rec.Body)
	}
}

func TestServerError(t *testing.T) {
	h, tok := setup(t, &fakeRepo{err: context.DeadlineExceeded})
	for _, c := range [][2]string{{"GET", "/api/blog"}, {"POST", "/api/blog"}, {"PUT", "/api/blog/1"}} {
		rec := do(h, c[0], c[1], tok, `{"videoId":"abc"}`)
		if rec.Code != 500 || msgOf(t, rec) != "Terjadi kesalahan pada server." {
			t.Errorf("%s %s = %d %s", c[0], c[1], rec.Code, rec.Body)
		}
	}
}
