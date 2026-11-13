//go:build integration

package sitecontent

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/app"
	"github.com/mrfroglab/portofolio-api-go/internal/auth"
	"github.com/mrfroglab/portofolio-api-go/internal/testutil"
)

func TestSiteContentCRUD(t *testing.T) {
	pool := testutil.DB(t)
	j := auth.NewJWT("secret-for-tests-secret-for-tests")
	tok, _ := j.Sign(1)
	mux := http.NewServeMux()
	Register(mux, app.Deps{Pool: pool, Guards: auth.Guards{JWT: j}})

	if rec := do(mux, "GET", "/api/site-content", "", ""); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "{}" {
		t.Fatalf("empty get: %d %s", rec.Code, rec.Body)
	}

	v1 := `{"eyebrow":"E","name":"N","paragraphs":["one"],"summaryTitle":"S","summary":[{"label":"l","value":"v"}]}`
	rec := do(mux, "PUT", "/api/site-content/about", tok, v1)
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	var put struct {
		Key   string
		Value struct {
			Name    string
			Summary []struct{ Accent bool }
		}
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &put)
	if put.Key != "about" || put.Value.Name != "N" || len(put.Value.Summary) != 1 {
		t.Errorf("put resp: %s", rec.Body)
	}

	var updated1 string
	if err := pool.QueryRow(t.Context(), `SELECT "updatedAt"::text FROM "SiteContent" WHERE "key"='about'`).Scan(&updated1); err != nil {
		t.Fatal(err)
	}

	// Upsert again (update path): value replaced wholesale, updatedAt advances.
	v2 := strings.Replace(v1, `"name":"N"`, `"name":"N2"`, 1)
	if rec = do(mux, "PUT", "/api/site-content/about", tok, v2); rec.Code != 200 {
		t.Fatalf("put2: %d %s", rec.Code, rec.Body)
	}
	var updated2 string
	_ = pool.QueryRow(t.Context(), `SELECT "updatedAt"::text FROM "SiteContent" WHERE "key"='about'`).Scan(&updated2)
	if updated2 <= updated1 {
		t.Errorf("updatedAt did not advance: %s -> %s", updated1, updated2)
	}

	rec = do(mux, "GET", "/api/site-content", "", "")
	var all map[string]struct{ Name string }
	_ = json.Unmarshal(rec.Body.Bytes(), &all)
	if len(all) != 1 || all["about"].Name != "N2" {
		t.Errorf("get: %s", rec.Body)
	}

	for i := 0; i < 2; i++ {
		rec = do(mux, "DELETE", "/api/site-content/about", tok, "")
		if rec.Code != 200 || msg(t, rec) != `Section "about" dihapus. Website memakai teks bawaan.` {
			t.Errorf("delete %d: %d %s", i, rec.Code, rec.Body)
		}
	}
	if rec = do(mux, "GET", "/api/site-content", "", ""); strings.TrimSpace(rec.Body.String()) != "{}" {
		t.Errorf("after delete: %s", rec.Body)
	}
}
