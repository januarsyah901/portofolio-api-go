package sitecontent

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const defaultsJSON = `{
 "experience": {"eyebrow":"Riwayat Kerja","title":"Pengalaman Profesional","items":[
  {"period":"2025","badge":"LSP UGM","accent":true,"title":"Pengembang Website","role":"Web Developer","description":"Deskripsi."},
  {"period":"2022 - 2024","badge":"MA","accent":false,"title":"PPDB","role":"Asisten IT","description":"Deskripsi 2."}]},
 "skills": {"eyebrow":"Keahlian","title":"Teknologi","categories":[
  {"label":"01 / FRONTEND","title":"Frontend","description":"Desc","skills":[{"name":"React","level":90},{"name":"TS","level":0}]},
  {"label":"02","title":"Backend","description":"Desc","skills":[]}]},
 "about": {"eyebrow":"Tentang","name":"Nama","paragraphs":["Satu","Dua"],"summaryTitle":"Ringkasan","summary":[
  {"label":"A","value":"B","accent":false},{"label":"C","value":"D","accent":true}]},
 "contact": {"ctaTitle":"Mari","ctaSubtitle":"Sub","brandDesc":"Brand","email":"a@b.co","location":"Sleman",
  "github":"https://github.com/x","linkedin":"https://www.linkedin.com/in/x","x":"https://x.com/x","instagram":"https://www.instagram.com/x/"}
}`

func defaults(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(defaultsJSON), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// fresh returns an independent copy of the defaults for key.
func fresh(t *testing.T, key string) map[string]any {
	return defaults(t)[key].(map[string]any)
}

func TestDefaultsPassUnchanged(t *testing.T) {
	for _, key := range Keys {
		v, msg, ok := Validate(key, fresh(t, key))
		if !ok {
			t.Fatalf("%s: %s", key, msg)
		}
		b, _ := json.Marshal(v)
		var got any
		_ = json.Unmarshal(b, &got)
		if !reflect.DeepEqual(got, defaults(t)[key]) {
			t.Errorf("%s changed:\n got %s", key, b)
		}
	}
}

func TestIsKey(t *testing.T) {
	for k, want := range map[string]bool{"skills": true, "contact": true, "hero": false, "__proto__": false, "": false} {
		if IsKey(k) != want {
			t.Errorf("IsKey(%q) != %v", k, want)
		}
	}
}

func msgOf(t *testing.T, key string, raw any) string {
	t.Helper()
	_, msg, ok := Validate(key, raw)
	if ok {
		t.Fatalf("%s: expected failure", key)
	}
	return msg
}

func TestNonObjectsAndMissing(t *testing.T) {
	for _, raw := range []any{nil, []any{}, "str", 1.0, true} {
		if got := msgOf(t, "experience", raw); got != `"experience" harus berupa objek.` {
			t.Errorf("got %q", got)
		}
	}
	if got := msgOf(t, "skills", map[string]any{}); got != `"eyebrow" harus berupa teks.` {
		t.Errorf("got %q", got)
	}
	if got := msgOf(t, "experience", map[string]any{"eyebrow": "x"}); got != `"title" harus berupa teks.` {
		t.Errorf("got %q", got)
	}
}

func TestDropsUnknownFieldsAndTrims(t *testing.T) {
	in := fresh(t, "experience")
	in["evil"] = 1.0
	in["items"].([]any)[0].(map[string]any)["extra"] = "x"
	in["title"] = "  Judul \u00a0\ufeff "
	v, _, ok := Validate("experience", in)
	if !ok {
		t.Fatal("expected ok")
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "evil") || strings.Contains(string(b), "extra") {
		t.Errorf("unknown fields kept: %s", b)
	}
	if v.(Experience).Title != "Judul" {
		t.Errorf("title = %q", v.(Experience).Title)
	}
}

func TestOutputFieldOrder(t *testing.T) {
	v, _, _ := Validate("experience", fresh(t, "experience"))
	b, _ := json.Marshal(v)
	want := `{"eyebrow":"Riwayat Kerja","title":"Pengalaman Profesional","items":[{"period":"2025","badge":"LSP UGM","accent":true,"title":"Pengembang Website","role":"Web Developer","description":"Deskripsi."}`
	if !strings.HasPrefix(string(b), want) {
		t.Errorf("order wrong: %s", b)
	}
	v, _, _ = Validate("contact", fresh(t, "contact"))
	b, _ = json.Marshal(v)
	if !strings.HasPrefix(string(b), `{"ctaTitle":"Mari","ctaSubtitle":"Sub","brandDesc":"Brand","email":"a@b.co","location":"Sleman","github":`) {
		t.Errorf("order wrong: %s", b)
	}
}

func TestEmptyListsSerialiseAsArrays(t *testing.T) {
	in := fresh(t, "experience")
	in["items"] = []any{}
	v, _, _ := Validate("experience", in)
	b, _ := json.Marshal(v)
	if !strings.HasSuffix(string(b), `"items":[]}`) {
		t.Errorf("got %s", b)
	}
}

func TestAccent(t *testing.T) {
	in := fresh(t, "experience")
	item := in["items"].([]any)[0].(map[string]any)
	delete(item, "accent")
	v, _, ok := Validate("experience", in)
	if !ok || v.(Experience).Items[0].Accent {
		t.Error("missing accent should default false")
	}
	for _, bad := range []any{"yes", nil, 1.0} {
		item["accent"] = bad
		if got := msgOf(t, "experience", in); got != `"items[0].accent" harus true atau false.` {
			t.Errorf("accent %v: %q", bad, got)
		}
	}
}

func TestSkillLevel(t *testing.T) {
	for _, level := range []any{101.0, -1.0, 50.5, "80", nil, true} {
		in := fresh(t, "skills")
		in["categories"].([]any)[0].(map[string]any)["skills"].([]any)[0].(map[string]any)["level"] = level
		if got := msgOf(t, "skills", in); got != `"categories[0].skills[0].level" harus bilangan bulat 0-100.` {
			t.Errorf("level %v: %q", level, got)
		}
	}
	in := fresh(t, "skills")
	in["categories"].([]any)[0].(map[string]any)["skills"].([]any)[0].(map[string]any)["level"] = 100.0
	if _, _, ok := Validate("skills", in); !ok {
		t.Error("100 should pass")
	}
}

func TestPathMessages(t *testing.T) {
	in := fresh(t, "experience")
	in["items"].([]any)[1].(map[string]any)["title"] = "   "
	if got := msgOf(t, "experience", in); got != `"items[1].title" tidak boleh kosong.` {
		t.Errorf("got %q", got)
	}
	in = fresh(t, "experience")
	in["items"] = []any{"x"}
	if got := msgOf(t, "experience", in); got != `"items[0]" harus berupa objek.` {
		t.Errorf("got %q", got)
	}
	in["items"] = "x"
	if got := msgOf(t, "experience", in); got != `"items" harus berupa daftar.` {
		t.Errorf("got %q", got)
	}
}

func TestAboutParagraphs(t *testing.T) {
	in := fresh(t, "about")
	in["paragraphs"] = []any{}
	if got := msgOf(t, "about", in); got != `"paragraphs" minimal 1 item.` {
		t.Errorf("got %q", got)
	}
	in["paragraphs"] = []any{"ok", 3.0}
	if got := msgOf(t, "about", in); got != `"paragraphs[1]" harus berupa teks.` {
		t.Errorf("got %q", got)
	}
	in["paragraphs"] = []any{strings.Repeat("x", 2001)}
	if got := msgOf(t, "about", in); got != `"paragraphs[0]" maksimal 2000 karakter.` {
		t.Errorf("got %q", got)
	}
}

func TestContactURLsAndEmail(t *testing.T) {
	for _, bad := range []string{"javascript:alert(1)", "http://x.com", "//x.com", "https://a b.com", "data:text/html,x", "https://", "ftp://x.com", "https://a b.com"} {
		in := fresh(t, "contact")
		in["github"] = bad
		if got := msgOf(t, "contact", in); got != `"github" harus berupa URL https://.` {
			t.Errorf("%q: %q", bad, got)
		}
	}
	for _, good := range []string{"https://github.com/x", "HTTPS://GITHUB.COM/x", "  https://github.com/x  "} {
		in := fresh(t, "contact")
		in["github"] = good
		if _, msg, ok := Validate("contact", in); !ok {
			t.Errorf("%q: %s", good, msg)
		}
	}
	for _, bad := range []string{"not-an-email", "a@b", "a b@c.d", "@b.c", "a@@b.c", "a@b.c d"} {
		in := fresh(t, "contact")
		in["email"] = bad
		if got := msgOf(t, "contact", in); got != `"email" bukan alamat email yang valid.` {
			t.Errorf("%q: %q", bad, got)
		}
	}
	in := fresh(t, "contact")
	in["x"] = "javascript:alert(1)"
	if got := msgOf(t, "contact", in); got != `"x" harus berupa URL https://.` {
		t.Errorf("got %q", got)
	}
}

func TestLimits(t *testing.T) {
	in := fresh(t, "experience")
	item := in["items"].([]any)[0]
	in["items"] = func() []any {
		l := make([]any, 21)
		for i := range l {
			l[i] = item
		}
		return l
	}()
	if got := msgOf(t, "experience", in); got != `"items" maksimal 20 item.` {
		t.Errorf("got %q", got)
	}
	// Every text limit: exactly max passes, max+1 fails.
	limits := []struct {
		key  string
		set  func(m map[string]any, v string)
		path string
		max  int
	}{
		{"experience", func(m map[string]any, v string) { m["eyebrow"] = v }, "eyebrow", 100},
		{"experience", func(m map[string]any, v string) { m["title"] = v }, "title", 200},
		{"experience", func(m map[string]any, v string) { m["items"].([]any)[0].(map[string]any)["period"] = v }, "items[0].period", 60},
		{"experience", func(m map[string]any, v string) { m["items"].([]any)[0].(map[string]any)["badge"] = v }, "items[0].badge", 80},
		{"experience", func(m map[string]any, v string) { m["items"].([]any)[0].(map[string]any)["role"] = v }, "items[0].role", 120},
		{"experience", func(m map[string]any, v string) { m["items"].([]any)[0].(map[string]any)["description"] = v }, "items[0].description", 2000},
		{"skills", func(m map[string]any, v string) { m["categories"].([]any)[0].(map[string]any)["label"] = v }, "categories[0].label", 60},
		{"skills", func(m map[string]any, v string) { m["categories"].([]any)[0].(map[string]any)["title"] = v }, "categories[0].title", 120},
		{"skills", func(m map[string]any, v string) { m["categories"].([]any)[0].(map[string]any)["description"] = v }, "categories[0].description", 500},
		{"skills", func(m map[string]any, v string) {
			m["categories"].([]any)[0].(map[string]any)["skills"].([]any)[0].(map[string]any)["name"] = v
		}, "categories[0].skills[0].name", 80},
		{"about", func(m map[string]any, v string) { m["name"] = v }, "name", 120},
		{"about", func(m map[string]any, v string) { m["summaryTitle"] = v }, "summaryTitle", 100},
		{"about", func(m map[string]any, v string) { m["summary"].([]any)[0].(map[string]any)["label"] = v }, "summary[0].label", 80},
		{"about", func(m map[string]any, v string) { m["summary"].([]any)[0].(map[string]any)["value"] = v }, "summary[0].value", 200},
		{"contact", func(m map[string]any, v string) { m["ctaTitle"] = v }, "ctaTitle", 120},
		{"contact", func(m map[string]any, v string) { m["ctaSubtitle"] = v }, "ctaSubtitle", 300},
		{"contact", func(m map[string]any, v string) { m["brandDesc"] = v }, "brandDesc", 500},
		{"contact", func(m map[string]any, v string) { m["location"] = v }, "location", 120},
		{"contact", func(m map[string]any, v string) { m["github"] = "https://x.co/" + v }, "github", 300},
	}
	for _, l := range limits {
		pad := func(n int) string {
			if l.path == "github" {
				return strings.Repeat("a", n-len("https://x.co/"))
			}
			return strings.Repeat("a", n)
		}
		in := fresh(t, l.key)
		l.set(in, pad(l.max))
		if _, msg, ok := Validate(l.key, in); !ok {
			t.Errorf("%s at max: %s", l.path, msg)
		}
		in = fresh(t, l.key)
		l.set(in, pad(l.max+1))
		want := `"` + l.path + `" maksimal ` + itoa(l.max) + ` karakter.`
		if got := msgOf(t, l.key, in); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
	// List caps.
	in = fresh(t, "about")
	in["summary"] = make([]any, 13)
	if got := msgOf(t, "about", in); got != `"summary" maksimal 12 item.` {
		t.Errorf("got %q", got)
	}
	in = fresh(t, "about")
	in["paragraphs"] = make([]any, 11)
	if got := msgOf(t, "about", in); got != `"paragraphs" maksimal 10 item.` {
		t.Errorf("got %q", got)
	}
	in = fresh(t, "skills")
	in["categories"] = make([]any, 13)
	if got := msgOf(t, "skills", in); got != `"categories" maksimal 12 item.` {
		t.Errorf("got %q", got)
	}
	in = fresh(t, "skills")
	in["categories"].([]any)[0].(map[string]any)["skills"] = make([]any, 13)
	if got := msgOf(t, "skills", in); got != `"categories[0].skills" maksimal 12 item.` {
		t.Errorf("got %q", got)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// JS .length counts UTF-16 code units: an emoji is 2, so 50 emoji = 100 units (ok), 51 = 102 (too long).
func TestEmojiLengthIsUTF16(t *testing.T) {
	if utf16Len("😀") != 2 || utf16Len("é") != 1 || utf16Len("a😀") != 3 {
		t.Fatal("utf16Len wrong")
	}
	in := fresh(t, "experience")
	in["eyebrow"] = strings.Repeat("😀", 50)
	if _, msg, ok := Validate("experience", in); !ok {
		t.Errorf("50 emoji (100 units) should pass: %s", msg)
	}
	in["eyebrow"] = strings.Repeat("😀", 51) // 51 runes but 102 units
	if got := msgOf(t, "experience", in); got != `"eyebrow" maksimal 100 karakter.` {
		t.Errorf("got %q", got)
	}
}
