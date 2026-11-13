package project

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

func fields(t *testing.T, s string) httpx.Fields {
	t.Helper()
	f := httpx.Fields{}
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		t.Fatalf("bad test json %q: %v", s, err)
	}
	return f
}

func sp(s string) *string { return &s }

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Cak MaD Editor", "cak-mad-editor"},
		{"Java Al-'Alim UGM", "java-al-alim-ugm"},
		{"Jama'ah Vokasi Al-'Alim (WordPress)", "jama-ah-vokasi-al-alim-wordpress"},
		{"  --Boros Token!! ", "boros-token"},
		{"***", ""},
		{"", ""},
		{"Héllo Wörld", "h-llo-w-rld"},
		{"v2.0", "v2-0"},
	}
	for _, tc := range tests {
		if got := Slugify(tc.in); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	tests := []struct {
		name  string
		taken []string
		want  string
	}{
		{"free", []string{"other"}, "cepaty"},
		{"none taken", nil, "cepaty"},
		{"first counter", []string{"cepaty"}, "cepaty-2"},
		{"skips used counters", []string{"cepaty", "cepaty-2", "cepaty-3"}, "cepaty-4"},
		{"fills gap", []string{"cepaty", "cepaty-3"}, "cepaty-2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := UniqueSlug("cepaty", tc.taken); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

const valid = `"title":"Cak MaD Editor","description":"Editor Markdown."`

func TestParseInputCreate(t *testing.T) {
	failures := []struct{ name, body, msg string }{
		{"empty body", `{}`, "Judul dan deskripsi proyek wajib diisi."},
		{"only title", `{"title":"x"}`, "Judul dan deskripsi proyek wajib diisi."},
		{"null title", `{"title":null,"description":"d"}`, `Field "title" harus berupa teks yang tidak kosong.`},
		{"blank description", `{"title":"t","description":"  "}`, `Field "description" harus berupa teks yang tidak kosong.`},
		{"category", `{` + valid + `,"category":"mobile"}`, `Field "category" harus salah satu dari: web, desktop, tool.`},
		{"status", `{` + valid + `,"status":"archived"}`, `Field "status" harus salah satu dari: live, open-source, internal.`},
		{"highlights string", `{` + valid + `,"highlights":"a"}`, `Field "highlights" harus berupa daftar teks.`},
		{"highlights number item", `{` + valid + `,"highlights":[1]}`, `Field "highlights" harus berupa daftar teks.`},
		{"featured string", `{` + valid + `,"featured":"true"}`, `Field "featured" harus true atau false.`},
		{"liveUrl number", `{` + valid + `,"liveUrl":5}`, `Field "liveUrl" harus berupa teks atau null.`},
		{"slug symbols", `{` + valid + `,"slug":"!!!"}`, `Field "slug" harus mengandung huruf atau angka.`},
		{"slug number", `{` + valid + `,"slug":5}`, `Field "slug" harus berupa teks atau null.`},
	}
	for _, tc := range failures {
		t.Run("fail/"+tc.name, func(t *testing.T) {
			_, msg := ParseInput(fields(t, tc.body), Create)
			if msg != tc.msg {
				t.Errorf("message = %q, want %q", msg, tc.msg)
			}
		})
	}

	t.Run("minimal", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{`+valid+`}`), Create)
		if msg != "" || *in.Title != "Cak MaD Editor" || *in.Description != "Editor Markdown." {
			t.Fatalf("got %+v %q", in, msg)
		}
		if in.Slug.Set || in.Tags != nil || in.Order != nil || in.Visible != nil || in.Featured != nil {
			t.Errorf("unexpected fields set: %+v", in)
		}
	})

	t.Run("every field", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{`+valid+`,"slug":"cak-mad-editor","category":"tool","status":"open-source",
			"year":"2026","role":"Solo developer","highlights":[" Ekspor PDF ",""],
			"releaseUrl":"https://example.com/releases","featured":true}`), Create)
		if msg != "" {
			t.Fatal(msg)
		}
		want := Input{
			Title: sp("Cak MaD Editor"), Description: sp("Editor Markdown."),
			Slug: Text{true, sp("cak-mad-editor")}, Category: sp("tool"), Status: sp("open-source"),
			Year: Text{true, sp("2026")}, Role: Text{true, sp("Solo developer")},
			Highlights: &[]string{"Ekspor PDF"}, ReleaseURL: Text{true, sp("https://example.com/releases")},
			Featured: new(bool),
		}
		*want.Featured = true
		if !reflect.DeepEqual(in, want) {
			t.Errorf("got %+v\nwant %+v", in, want)
		}
	})

	t.Run("empty nullable text becomes null", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{`+valid+`,"liveUrl":"","githubUrl":"  ","role":null,"year":" 2026 "}`), Create)
		if msg != "" {
			t.Fatal(msg)
		}
		for name, v := range map[string]Text{"liveUrl": in.LiveURL, "githubUrl": in.GithubURL, "role": in.Role} {
			if !v.Set || v.Val != nil {
				t.Errorf("%s = %+v, want Set null", name, v)
			}
		}
		if !in.Year.Set || *in.Year.Val != "2026" {
			t.Errorf("year = %+v", in.Year)
		}
	})

	t.Run("slug normalised, empty dropped", func(t *testing.T) {
		in, _ := ParseInput(fields(t, `{`+valid+`,"slug":"My Slug"}`), Create)
		if !in.Slug.Set || *in.Slug.Val != "my-slug" {
			t.Errorf("slug = %+v", in.Slug)
		}
		for _, s := range []string{`"slug":""`, `"slug":null`, `"slug":"  "`} {
			in, msg := ParseInput(fields(t, `{`+valid+`,`+s+`}`), Create)
			if msg != "" || in.Slug.Set {
				t.Errorf("%s: slug = %+v msg=%q, want dropped", s, in.Slug, msg)
			}
		}
	})

	t.Run("title and description trimmed", func(t *testing.T) {
		in, _ := ParseInput(fields(t, `{"title":"  A  ","description":"\n B "}`), Create)
		if *in.Title != "A" || *in.Description != "B" {
			t.Errorf("got %q %q", *in.Title, *in.Description)
		}
	})
}

func TestParseInputLenient(t *testing.T) {
	t.Run("wrong types ignored", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{`+valid+`,"tags":"x","order":"1","visible":"no"}`), Create)
		if msg != "" || in.Tags != nil || in.Order != nil || in.Visible != nil {
			t.Errorf("got %+v %q", in, msg)
		}
	})
	t.Run("right types applied", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{`+valid+`,"tags":["Swift"],"order":3,"visible":false}`), Create)
		if msg != "" || !reflect.DeepEqual(*in.Tags, []string{"Swift"}) || *in.Order != 3 || *in.Visible {
			t.Errorf("got %+v %q", in, msg)
		}
	})
	t.Run("tags keep string items only", func(t *testing.T) {
		in, _ := ParseInput(fields(t, `{`+valid+`,"tags":["a",1,null,"b",{"x":1}]}`), Create)
		if !reflect.DeepEqual(*in.Tags, []string{"a", "b"}) {
			t.Errorf("tags = %v", *in.Tags)
		}
	})
	t.Run("empty tags array is applied", func(t *testing.T) {
		in, _ := ParseInput(fields(t, `{`+valid+`,"tags":[]}`), Create)
		if in.Tags == nil || len(*in.Tags) != 0 {
			t.Errorf("tags = %v", in.Tags)
		}
	})
	t.Run("order truncated", func(t *testing.T) {
		for body, want := range map[string]int{`2.9`: 2, `-2.9`: -2, `0`: 0, `7`: 7} {
			in, _ := ParseInput(fields(t, `{`+valid+`,"order":`+body+`}`), Create)
			if in.Order == nil || *in.Order != want {
				t.Errorf("order %s = %v, want %d", body, in.Order, want)
			}
		}
	})
	t.Run("order out of int32 range ignored", func(t *testing.T) {
		in, _ := ParseInput(fields(t, `{`+valid+`,"order":1e12}`), Create)
		if in.Order != nil {
			t.Errorf("order = %d", *in.Order)
		}
	})
}

func TestParseInputUpdate(t *testing.T) {
	t.Run("empty body", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{}`), Update)
		if msg != "" || !reflect.DeepEqual(in, Input{}) {
			t.Errorf("got %+v %q", in, msg)
		}
	})
	t.Run("partial", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{"featured":false}`), Update)
		if msg != "" || in.Featured == nil || *in.Featured || in.Title != nil {
			t.Errorf("got %+v %q", in, msg)
		}
	})
	t.Run("empty urls become null", func(t *testing.T) {
		in, _ := ParseInput(fields(t, `{"liveUrl":"","imageUrl":""}`), Update)
		if !in.LiveURL.Set || in.LiveURL.Val != nil || !in.ImageURL.Set || in.ImageURL.Val != nil {
			t.Errorf("got %+v", in)
		}
	})
	t.Run("empty slug clears", func(t *testing.T) {
		for _, s := range []string{`""`, `null`} {
			in, msg := ParseInput(fields(t, `{"slug":`+s+`}`), Update)
			if msg != "" || !in.Slug.Set || in.Slug.Val != nil {
				t.Errorf("slug %s: %+v %q", s, in.Slug, msg)
			}
		}
	})
	t.Run("blank title rejected", func(t *testing.T) {
		if _, msg := ParseInput(fields(t, `{"title":" "}`), Update); msg == "" {
			t.Error("expected failure")
		}
	})
}
