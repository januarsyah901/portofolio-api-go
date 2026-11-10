//go:build integration

package blog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
	"github.com/januarsyah901/portofolio-api-go/internal/auth"
	"github.com/januarsyah901/portofolio-api-go/internal/testutil"
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
	itoa := func(n int) string { b, _ := json.Marshal(n); return string(b) }

	var list []Video
	if c := call("GET", "/api/blog", "", &list); c != 200 || list == nil || len(list) != 0 {
		t.Fatalf("empty list: %d %v", c, list)
	}

	var a, b, c3 Video
	if c := call("POST", "/api/blog", `{"videoId":"111","caption":"hi","tags":["x","y"],"order":2}`, &a); c != 201 {
		t.Fatalf("create a: %d", c)
	}
	if a.ID == 0 || a.VideoID != "111" || a.Caption != "hi" || len(a.Tags) != 2 || a.Order != 2 || !a.Visible {
		t.Fatalf("a = %+v", a)
	}
	if c := call("POST", "/api/blog", `{"videoId":"222","order":1}`, &b); c != 201 || b.Caption != "" || b.Tags == nil || len(b.Tags) != 0 {
		t.Fatalf("create b: %d %+v", c, b)
	}
	if c := call("POST", "/api/blog", `{"videoId":"333","order":3,"visible":false}`, &c3); c != 201 {
		t.Fatalf("create c: %d", c)
	}

	call("GET", "/api/blog", "", &list)
	if len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID { // order asc, hidden excluded
		t.Fatalf("public list = %+v", list)
	}
	call("GET", "/api/blog/admin-list", "", &list)
	if len(list) != 3 || list[2].ID != c3.ID {
		t.Fatalf("admin list = %+v", list)
	}

	var up Video
	if c := call("PUT", "/api/blog/"+itoa(a.ID), `{"caption":"new","tags":[],"visible":false}`, &up); c != 200 {
		t.Fatalf("update: %d", c)
	}
	if up.Caption != "new" || up.Tags == nil || len(up.Tags) != 0 || up.Visible || up.VideoID != "111" || up.Order != 2 {
		t.Fatalf("updated = %+v", up)
	}
	var msg struct{ Message string }
	if c := call("PUT", "/api/blog/"+itoa(a.ID), `{"videoId":null}`, &msg); c != 400 {
		t.Fatalf("null videoId: %d", c)
	}
	if c := call("PUT", "/api/blog/9999", `{}`, &msg); c != 404 || msg.Message != MsgNotFound {
		t.Fatalf("missing: %d %q", c, msg.Message)
	}
	if c := call("DELETE", "/api/blog/"+itoa(a.ID), "", &msg); c != 200 || msg.Message != MsgDeleted {
		t.Fatalf("delete: %d %q", c, msg.Message)
	}
	if c := call("DELETE", "/api/blog/"+itoa(a.ID), "", &msg); c != 404 {
		t.Fatalf("delete again: %d", c)
	}
}
