//go:build integration

package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/db"
	"github.com/mrfroglab/portofolio-api-go/internal/testutil"
)

func newIntegrationHandler(t *testing.T) (*Handler, *pgRepo) {
	t.Helper()
	repo := newPgRepo(testutil.DB(t))
	return NewHandler(repo), repo
}

func createOK(t *testing.T, h *Handler, body string) Project {
	t.Helper()
	rec := serve(h, "POST", "/api/projects", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s: %d %s", body, rec.Code, rec.Body.String())
	}
	var p Project
	decode(t, rec.Body.Bytes(), &p)
	return p
}

func TestIntegrationCreateAutoSlugAndConflict(t *testing.T) {
	h, _ := newIntegrationHandler(t)

	a := createOK(t, h, `{"title":"Cak MaD Editor","description":"d"}`)
	if a.Slug == nil || *a.Slug != "cak-mad-editor" {
		t.Fatalf("slug = %v", a.Slug)
	}
	if a.Category != "web" || a.Status != "live" || !a.Visible || a.Featured || a.Order != 0 ||
		a.Tags == nil || a.Highlights == nil || a.ImageURL != nil {
		t.Errorf("defaults wrong: %+v", a)
	}
	b := createOK(t, h, `{"title":"Cak MaD Editor","description":"d2"}`)
	if b.Slug == nil || *b.Slug != "cak-mad-editor-2" {
		t.Fatalf("duplicate title slug = %v, want cak-mad-editor-2", b.Slug)
	}
	c := createOK(t, h, `{"title":"Cak MaD Editor","description":"d3"}`)
	if *c.Slug != "cak-mad-editor-3" {
		t.Fatalf("third slug = %v", *c.Slug)
	}

	rec := serve(h, "POST", "/api/projects", `{"title":"Other","description":"d","slug":"Cak MaD Editor"}`)
	if rec.Code != http.StatusConflict || msgOf(t, rec) != "Slug sudah dipakai proyek lain." {
		t.Fatalf("explicit duplicate slug: %d %s", rec.Code, rec.Body.String())
	}
	// A failed insert must not leave a row behind.
	rec = serve(h, "GET", "/api/projects/admin-list", "")
	var all []Project
	decode(t, rec.Body.Bytes(), &all)
	if len(all) != 3 {
		t.Errorf("rows = %d, want 3", len(all))
	}
}

func TestIntegrationCRUDRoundTrip(t *testing.T) {
	h, _ := newIntegrationHandler(t)
	p := createOK(t, h, `{"title":"Alpha","description":"d","tags":["Go","SQL"],"highlights":[" one ",""],
		"liveUrl":"https://x.dev","year":"2026","role":"Dev","releaseUrl":"https://x.dev/r","featured":true,
		"order":4.7,"visible":true,"category":"tool","status":"open-source"}`)
	if len(p.Tags) != 2 || len(p.Highlights) != 1 || p.Highlights[0] != "one" || p.Order != 4 ||
		!p.Featured || p.Category != "tool" || p.Status != "open-source" ||
		p.LiveURL == nil || *p.LiveURL != "https://x.dev" {
		t.Fatalf("created = %+v", p)
	}

	// public read by slug
	rec := serve(h, "GET", "/api/projects/alpha", "")
	if rec.Code != 200 {
		t.Fatalf("get by slug: %d", rec.Code)
	}

	// partial update leaves other fields alone and bumps updatedAt
	rec = serve(h, "PUT", "/api/projects/1", `{"title":"Alpha 2","liveUrl":"","visible":false,"slug":""}`)
	if rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	var u Project
	decode(t, rec.Body.Bytes(), &u)
	if u.Title != "Alpha 2" || u.LiveURL != nil || u.Visible || u.Slug != nil ||
		u.Description != "d" || len(u.Tags) != 2 || u.Year == nil || *u.Year != "2026" {
		t.Fatalf("updated = %+v", u)
	}
	if u.UpdatedAt.Before(p.UpdatedAt.Time) || !u.CreatedAt.Equal(p.CreatedAt.Time) {
		t.Errorf("timestamps: created %v->%v updated %v->%v", p.CreatedAt, u.CreatedAt, p.UpdatedAt, u.UpdatedAt)
	}

	// hidden → not in public list / by slug
	rec = serve(h, "GET", "/api/projects", "")
	var pub []Project
	decode(t, rec.Body.Bytes(), &pub)
	if len(pub) != 0 {
		t.Errorf("public list = %d, want 0", len(pub))
	}

	// update conflict
	createOK(t, h, `{"title":"Beta","description":"d"}`)
	rec = serve(h, "PUT", "/api/projects/1", `{"slug":"beta"}`)
	if rec.Code != 409 {
		t.Errorf("update slug conflict: %d", rec.Code)
	}
	if rec = serve(h, "PUT", "/api/projects/999", `{"title":"x"}`); rec.Code != 404 {
		t.Errorf("update missing: %d", rec.Code)
	}

	// delete
	if rec = serve(h, "DELETE", "/api/projects/1", ""); rec.Code != 200 {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec = serve(h, "DELETE", "/api/projects/1", ""); rec.Code != 404 {
		t.Errorf("delete again: %d", rec.Code)
	}
}

func TestIntegrationPublicOrdering(t *testing.T) {
	h, _ := newIntegrationHandler(t)
	createOK(t, h, `{"title":"A","description":"d","order":2}`)
	createOK(t, h, `{"title":"B","description":"d","order":1}`)
	createOK(t, h, `{"title":"C","description":"d","order":3,"featured":true}`)
	createOK(t, h, `{"title":"D","description":"d","order":0,"visible":false}`)

	var pub, adm []Project
	decode(t, serve(h, "GET", "/api/projects", "").Body.Bytes(), &pub)
	decode(t, serve(h, "GET", "/api/projects/admin-list", "").Body.Bytes(), &adm)
	titles := func(ps []Project) string {
		s := ""
		for _, p := range ps {
			s += p.Title
		}
		return s
	}
	if got := titles(pub); got != "CBA" {
		t.Errorf("public order = %s, want CBA", got)
	}
	if got := titles(adm); got != "DBAC" {
		t.Errorf("admin order = %s, want DBAC", got)
	}
}

func orders(t *testing.T, h *Handler) map[int]int {
	t.Helper()
	var adm []Project
	decode(t, serve(h, "GET", "/api/projects/admin-list", "").Body.Bytes(), &adm)
	m := map[int]int{}
	for _, p := range adm {
		m[p.ID] = p.Order
	}
	return m
}

func TestIntegrationReorder(t *testing.T) {
	h, _ := newIntegrationHandler(t)
	for _, title := range []string{"A", "B", "C"} {
		createOK(t, h, `{"title":"`+title+`","description":"d","order":10}`)
	}

	rec := serve(h, "PUT", "/api/projects/reorder", `{"orderedIds":[3,"1",2]}`)
	if rec.Code != 200 || msgOf(t, rec) != "Urutan proyek berhasil diperbarui." {
		t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
	}
	if got := orders(t, h); got[3] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("orders = %v", got)
	}

	// Unknown id after valid ones: 400 and the earlier updates roll back.
	rec = serve(h, "PUT", "/api/projects/reorder", `{"orderedIds":[1,2,999,3]}`)
	if rec.Code != 400 || msgOf(t, rec) != "ID proyek tidak ditemukan: 999." {
		t.Fatalf("rollback case: %d %s", rec.Code, rec.Body.String())
	}
	if got := orders(t, h); got[3] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("orders changed after failed reorder: %v", got)
	}
}

func TestIntegrationRepoNotFoundAndUnknownID(t *testing.T) {
	_, repo := newIntegrationHandler(t)
	ctx := context.Background()
	if _, err := repo.GetVisibleBySlug(ctx, "x"); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("GetVisibleBySlug: %v", err)
	}
	var unknown *UnknownIDError
	if err := repo.Reorder(ctx, []int{5}); !errors.As(err, &unknown) || unknown.ID != 5 {
		t.Errorf("Reorder: %v", err)
	}
}

func decode(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
}
