package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/januarsyah901/portofolio-api-go/internal/db"
)

type fakeRepo struct {
	projects []Project
	nextID   int
	err      error // returned by every call when set
	created  Input
	updated  Input
	reorder  []int
}

func newFake() *fakeRepo {
	slug := "alpha"
	return &fakeRepo{nextID: 2, projects: []Project{{
		ID: 1, Title: "Alpha", Description: "d", Slug: &slug, Tags: []string{}, Highlights: []string{},
		Category: "web", Status: "live", Visible: true,
		CreatedAt: Timestamp{time.Date(2026, 11, 10, 12, 0, 0, 5e6, time.UTC)},
	}}}
}

func (f *fakeRepo) ListVisible(context.Context) ([]Project, error) {
	out := []Project{}
	for _, p := range f.projects {
		if p.Visible {
			out = append(out, p)
		}
	}
	return out, f.err
}
func (f *fakeRepo) ListAll(context.Context) ([]Project, error) {
	return append([]Project{}, f.projects...), f.err
}
func (f *fakeRepo) GetVisibleBySlug(_ context.Context, slug string) (Project, error) {
	if f.err != nil {
		return Project{}, f.err
	}
	for _, p := range f.projects {
		if p.Visible && p.Slug != nil && *p.Slug == slug {
			return p, nil
		}
	}
	return Project{}, db.ErrNotFound
}
func (f *fakeRepo) SlugsWithPrefix(_ context.Context, prefix string) ([]string, error) {
	out := []string{}
	for _, p := range f.projects {
		if p.Slug != nil && strings.HasPrefix(*p.Slug, prefix) {
			out = append(out, *p.Slug)
		}
	}
	return out, f.err
}
func (f *fakeRepo) Create(_ context.Context, in Input) (Project, error) {
	f.created = in
	if f.err != nil {
		return Project{}, f.err
	}
	p := Project{ID: f.nextID, Title: *in.Title, Description: *in.Description, Slug: in.Slug.Val,
		Tags: []string{}, Highlights: []string{}, Category: "web", Status: "live", Visible: true}
	f.nextID++
	return p, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input) (Project, error) {
	f.updated = in
	if f.err != nil {
		return Project{}, f.err
	}
	for _, p := range f.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, db.ErrNotFound
}
func (f *fakeRepo) Delete(_ context.Context, id int) error {
	if f.err != nil {
		return f.err
	}
	for _, p := range f.projects {
		if p.ID == id {
			return nil
		}
	}
	return db.ErrNotFound
}
func (f *fakeRepo) Reorder(_ context.Context, ids []int) error {
	f.reorder = ids
	if f.err != nil {
		return f.err
	}
	for _, id := range ids {
		found := false
		for _, p := range f.projects {
			found = found || p.ID == id
		}
		if !found {
			return &UnknownIDError{ID: id}
		}
	}
	return nil
}

var uniqueErr = &pgconn.PgError{Code: "23505"}

// serve routes through the same patterns as Register (without guards).
func serve(h *Handler, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", h.listPublic)
	mux.HandleFunc("GET /api/projects/admin-list", h.listAdmin)
	mux.HandleFunc("POST /api/projects", h.create)
	mux.HandleFunc("PUT /api/projects/reorder", h.reorder)
	mux.HandleFunc("GET /api/projects/{slug}", h.getBySlug)
	mux.HandleFunc("PUT /api/projects/{id}", h.update)
	mux.HandleFunc("DELETE /api/projects/{id}", h.delete)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
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

func TestHandlerStatusAndMessages(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeRepo)
		method string
		path   string
		body   string
		status int
		msg    string
	}{
		{"list public", nil, "GET", "/api/projects", "", 200, ""},
		// The repo would fail if reached: an id beyond int4 must be answered without a query.
		{"update id beyond int32", func(f *fakeRepo) { f.err = errors.New("boom") }, "PUT", "/api/projects/9999999999", `{"title":"x"}`, 404, "Proyek tidak ditemukan."},
		{"delete id beyond int32", func(f *fakeRepo) { f.err = errors.New("boom") }, "DELETE", "/api/projects/9999999999", "", 404, "Proyek tidak ditemukan."},
		{"list public db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "GET", "/api/projects", "", 500, "Terjadi kesalahan pada server."},
		{"list admin", nil, "GET", "/api/projects/admin-list", "", 200, ""},
		{"list admin db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "GET", "/api/projects/admin-list", "", 500, "Terjadi kesalahan pada server."},
		{"get by slug", nil, "GET", "/api/projects/alpha", "", 200, ""},
		{"get by slug missing", nil, "GET", "/api/projects/nope", "", 404, "Proyek tidak ditemukan."},
		{"get by slug hidden", func(f *fakeRepo) { f.projects[0].Visible = false }, "GET", "/api/projects/alpha", "", 404, "Proyek tidak ditemukan."},
		{"get by slug db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "GET", "/api/projects/alpha", "", 500, "Terjadi kesalahan pada server."},

		{"create ok", nil, "POST", "/api/projects", `{"title":"Beta","description":"d"}`, 201, ""},
		{"create missing fields", nil, "POST", "/api/projects", `{}`, 400, "Judul dan deskripsi proyek wajib diisi."},
		{"create invalid json", nil, "POST", "/api/projects", `{`, 400, "Body request harus berupa JSON yang valid."},
		{"create bad category", nil, "POST", "/api/projects", `{"title":"a","description":"d","category":"x"}`, 400, `Field "category" harus salah satu dari: web, desktop, tool.`},
		{"create slug conflict", func(f *fakeRepo) { f.err = uniqueErr }, "POST", "/api/projects", `{"title":"a","description":"d","slug":"alpha"}`, 409, "Slug sudah dipakai proyek lain."},
		{"create db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "POST", "/api/projects", `{"title":"a","description":"d"}`, 500, "Terjadi kesalahan pada server."},

		{"reorder ok", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[1]}`, 200, "Urutan proyek berhasil diperbarui."},
		{"reorder numeric string", nil, "PUT", "/api/projects/reorder", `{"orderedIds":["1"]}`, 200, "Urutan proyek berhasil diperbarui."},
		{"reorder empty", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[]}`, 200, "Urutan proyek berhasil diperbarui."},
		{"reorder not array", nil, "PUT", "/api/projects/reorder", `{"orderedIds":"1"}`, 400, "orderedIds harus berupa array dari ID proyek."},
		{"reorder missing", nil, "PUT", "/api/projects/reorder", `{}`, 400, "orderedIds harus berupa array dari ID proyek."},
		{"reorder unknown id", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[1,99]}`, 400, "ID proyek tidak ditemukan: 99."},
		{"reorder zero", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[0]}`, 400, "ID proyek tidak valid."},
		{"reorder negative", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[-1]}`, 400, "ID proyek tidak valid."},
		{"reorder fraction", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[1.5]}`, 400, "ID proyek tidak valid."},
		{"reorder text", nil, "PUT", "/api/projects/reorder", `{"orderedIds":["abc"]}`, 400, "ID proyek tidak valid."},
		{"reorder empty string", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[""]}`, 400, "ID proyek tidak valid."},
		{"reorder hex string", nil, "PUT", "/api/projects/reorder", `{"orderedIds":["0x10"]}`, 400, "ID proyek tidak valid."},
		{"reorder null", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[null]}`, 400, "ID proyek tidak valid."},
		{"reorder bool", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[true]}`, 400, "ID proyek tidak valid."},
		{"reorder too big", nil, "PUT", "/api/projects/reorder", `{"orderedIds":[99999999999]}`, 400, "ID proyek tidak valid."},
		{"reorder db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "PUT", "/api/projects/reorder", `{"orderedIds":[1]}`, 500, "Terjadi kesalahan pada server."},

		{"update ok", nil, "PUT", "/api/projects/1", `{"title":"New"}`, 200, ""},
		{"update empty body", nil, "PUT", "/api/projects/1", ``, 200, ""},
		{"update bad id", nil, "PUT", "/api/projects/abc", `{}`, 400, "ID proyek tidak valid."},
		{"update invalid body", nil, "PUT", "/api/projects/1", `{"title":" "}`, 400, `Field "title" harus berupa teks yang tidak kosong.`},
		{"update missing", nil, "PUT", "/api/projects/42", `{"title":"x"}`, 404, "Proyek tidak ditemukan."},
		{"update slug conflict", func(f *fakeRepo) { f.err = uniqueErr }, "PUT", "/api/projects/1", `{"slug":"beta"}`, 409, "Slug sudah dipakai proyek lain."},
		{"update db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "PUT", "/api/projects/1", `{}`, 500, "Terjadi kesalahan pada server."},

		{"delete ok", nil, "DELETE", "/api/projects/1", "", 200, "Proyek berhasil dihapus."},
		{"delete bad id", nil, "DELETE", "/api/projects/abc", "", 400, "ID proyek tidak valid."},
		{"delete missing", nil, "DELETE", "/api/projects/42", "", 404, "Proyek tidak ditemukan."},
		{"delete db error", func(f *fakeRepo) { f.err = errors.New("boom") }, "DELETE", "/api/projects/1", "", 500, "Terjadi kesalahan pada server."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			if tc.mutate != nil {
				tc.mutate(f)
			}
			rec := serve(NewHandler(f), tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body.String())
			}
			if tc.msg != "" {
				if got := msgOf(t, rec); got != tc.msg {
					t.Errorf("message = %q, want %q", got, tc.msg)
				}
			}
		})
	}
}

func TestCreateGeneratesUniqueSlug(t *testing.T) {
	f := newFake() // already has slug "alpha"
	for body, want := range map[string]string{
		`{"title":"Alpha","description":"d"}`:                 "alpha-2",
		`{"title":"Brand New!","description":"d"}`:            "brand-new",
		`{"title":"Alpha","description":"d","slug":"My Own"}`: "my-own",
		`{"title":"Alpha","description":"d","slug":""}`:       "alpha-2",
	} {
		rec := serve(NewHandler(f), "POST", "/api/projects", body)
		if rec.Code != 201 {
			t.Fatalf("%s: status %d %s", body, rec.Code, rec.Body.String())
		}
		if f.created.Slug.Val == nil || *f.created.Slug.Val != want {
			t.Errorf("%s: slug = %v, want %s", body, f.created.Slug.Val, want)
		}
	}
	// A title with no slug characters yields a null slug.
	serve(NewHandler(f), "POST", "/api/projects", `{"title":"***","description":"d"}`)
	if f.created.Slug.Val != nil {
		t.Errorf("slug = %v, want nil", *f.created.Slug.Val)
	}
}

func TestReorderPassesIDsInOrder(t *testing.T) {
	f := newFake()
	f.projects = append(f.projects, Project{ID: 5}, Project{ID: 7})
	rec := serve(NewHandler(f), "PUT", "/api/projects/reorder", `{"orderedIds":[7,"5",1]}`)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if len(f.reorder) != 3 || f.reorder[0] != 7 || f.reorder[1] != 5 || f.reorder[2] != 1 {
		t.Errorf("ids = %v", f.reorder)
	}
}

func TestJSONShape(t *testing.T) {
	rec := serve(NewHandler(newFake()), "GET", "/api/projects", "")
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil || len(arr) != 1 {
		t.Fatalf("body %s: %v", rec.Body.String(), err)
	}
	keys := []string{"id", "title", "description", "imageUrl", "liveUrl", "githubUrl", "tags", "slug",
		"category", "status", "year", "role", "highlights", "releaseUrl", "featured", "order", "visible",
		"createdAt", "updatedAt"}
	for _, k := range keys {
		if _, ok := arr[0][k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if len(arr[0]) != len(keys) {
		t.Errorf("got %d keys, want %d", len(arr[0]), len(keys))
	}
	for k, want := range map[string]string{"imageUrl": "null", "tags": "[]", "highlights": "[]",
		"createdAt": `"2026-11-10T12:00:00.005Z"`} {
		if got := string(arr[0][k]); got != want {
			t.Errorf("%s = %s, want %s", k, got, want)
		}
	}
}

func TestListEmptyIsArray(t *testing.T) {
	f := newFake()
	f.projects = nil
	rec := serve(NewHandler(f), "GET", "/api/projects", "")
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %s, want []", got)
	}
}

func TestUpdateOnlyProvidedFields(t *testing.T) {
	f := newFake()
	serve(NewHandler(f), "PUT", "/api/projects/1", `{"title":"New","liveUrl":"","visible":false}`)
	u := f.updated
	if u.Title == nil || *u.Title != "New" || !u.LiveURL.Set || u.LiveURL.Val != nil ||
		u.Visible == nil || *u.Visible || u.Description != nil || u.GithubURL.Set || u.Slug.Set {
		t.Errorf("unexpected input %+v", u)
	}
}

func decode(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
}
