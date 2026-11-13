//go:build integration

package testimonial

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/testutil"
)

func TestIntegrationCRUD(t *testing.T) {
	pool := testutil.DB(t)
	jwt := auth.NewJWT("0123456789abcdef0123456789abcdef")
	tok, _ := jwt.Sign(1)
	mux := http.NewServeMux()
	Register(mux, app.Deps{Pool: pool, JWT: jwt, Guards: auth.Guards{JWT: jwt}})

	call := func(method, path, body string, out any) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if out != nil {
			if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
				t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
			}
		}
		return rec.Code
	}

	var list []Testimonial
	if c := call("GET", "/api/testimonials", "", &list); c != 200 || list == nil || len(list) != 0 {
		t.Fatalf("empty list: %d %v", c, list)
	}

	var a, b Testimonial
	if c := call("POST", "/api/testimonials", `{"name":"A","role":"Dev","text":"first","company":"Co","rating":4}`, &a); c != 201 {
		t.Fatalf("create a: %d", c)
	}
	if a.ID == 0 || a.Company == nil || *a.Company != "Co" || a.Rating != 4 || !a.Visible || a.AvatarURL != nil || a.CreatedAt.IsZero() {
		t.Fatalf("a = %+v", a)
	}
	if c := call("POST", "/api/testimonials", `{"name":"B","role":"Dev","text":"second","visible":false}`, &b); c != 201 || b.Rating != 5 || b.Visible {
		t.Fatalf("create b: %d %+v", c, b)
	}

	call("GET", "/api/testimonials", "", &list)
	if len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("public list = %+v", list)
	}
	call("GET", "/api/testimonials/admin-list", "", &list)
	if len(list) != 2 || list[0].ID != b.ID { // createdAt desc: newest first
		t.Fatalf("admin list = %+v", list)
	}

	var up Testimonial
	if c := call("PUT", "/api/testimonials/"+itoa(a.ID), `{"name":"A2","company":null,"visible":false}`, &up); c != 200 {
		t.Fatalf("update: %d", c)
	}
	if up.Name != "A2" || up.Company != nil || up.Visible || up.Text != "first" || up.Rating != 4 || !up.CreatedAt.Equal(a.CreatedAt) {
		t.Fatalf("updated = %+v", up)
	}
	var msg struct{ Message string }
	if c := call("PUT", "/api/testimonials/"+itoa(a.ID), `{"name":null}`, &msg); c != 400 {
		t.Fatalf("null name: %d", c)
	}
	if c := call("PUT", "/api/testimonials/9999", `{}`, &msg); c != 404 || msg.Message != MsgNotFound {
		t.Fatalf("missing: %d %q", c, msg.Message)
	}

	if c := call("DELETE", "/api/testimonials/"+itoa(a.ID), "", &msg); c != 200 || msg.Message != MsgDeleted {
		t.Fatalf("delete: %d %q", c, msg.Message)
	}
	if c := call("DELETE", "/api/testimonials/"+itoa(a.ID), "", &msg); c != 404 {
		t.Fatalf("delete again: %d", c)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
