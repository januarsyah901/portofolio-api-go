//go:build integration

package orgexp

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

	var list []Entry
	if c := call("GET", "/api/org-experience", "", &list); c != 200 || list == nil || len(list) != 0 {
		t.Fatalf("empty list: %d %v", c, list)
	}

	var a, b, c3 Entry
	if c := call("POST", "/api/org-experience", ` {"period":" 2026 ","organization":"GSA","role":"Ambassador"}`, &a); c != 201 {
		t.Fatalf("create a: %d", c)
	}
	if a.ID == 0 || a.Period != "2026" || a.Description != "" || a.Type != "organization" || a.Logo != nil ||
		a.Order != 0 || !a.Visible || a.Highlights == nil || len(a.Highlights) != 0 {
		t.Fatalf("a = %+v", a)
	}
	if c := call("POST", "/api/org-experience", `{"period":"p","organization":"o","role":"r","type":"event","highlights":[" h ",""],"logo":"/l.png","order":0}`, &b); c != 201 {
		t.Fatalf("create b: %d", c)
	}
	if b.Type != "event" || len(b.Highlights) != 1 || b.Highlights[0] != "h" || b.Logo == nil || *b.Logo != "/l.png" {
		t.Fatalf("b = %+v", b)
	}
	if c := call("POST", "/api/org-experience", `{"period":"p","organization":"o","role":"r","order":-1,"visible":false}`, &c3); c != 201 {
		t.Fatalf("create c: %d", c)
	}

	call("GET", "/api/org-experience", "", &list)
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID { // order asc, id asc; hidden excluded
		t.Fatalf("public list = %+v", list)
	}
	call("GET", "/api/org-experience/admin-list", "", &list)
	if len(list) != 3 || list[0].ID != c3.ID || list[1].ID != a.ID || list[2].ID != b.ID {
		t.Fatalf("admin list = %+v", list)
	}

	var up Entry
	if c := call("PUT", "/api/org-experience/"+itoa(b.ID), `{"role":"Ketua","logo":null,"visible":false}`, &up); c != 200 {
		t.Fatalf("update: %d", c)
	}
	if up.Role != "Ketua" || up.Logo != nil || up.Visible || up.Type != "event" || len(up.Highlights) != 1 {
		t.Fatalf("updated = %+v", up)
	}
	var msg struct{ Message string }
	if c := call("PUT", "/api/org-experience/x", `{}`, &msg); c != 400 || msg.Message != MsgBadID {
		t.Fatalf("bad id: %d %q", c, msg.Message)
	}
	if c := call("PUT", "/api/org-experience/"+itoa(b.ID), `{"type":"club"}`, &msg); c != 400 {
		t.Fatalf("bad type: %d", c)
	}
	if c := call("PUT", "/api/org-experience/9999", `{}`, &msg); c != 404 || msg.Message != MsgNotFound {
		t.Fatalf("missing: %d %q", c, msg.Message)
	}
	if c := call("DELETE", "/api/org-experience/"+itoa(a.ID), "", &msg); c != 200 || msg.Message != MsgDeleted {
		t.Fatalf("delete: %d %q", c, msg.Message)
	}
	if c := call("DELETE", "/api/org-experience/"+itoa(a.ID), "", &msg); c != 404 {
		t.Fatalf("delete again: %d", c)
	}
}
