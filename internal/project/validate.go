package project

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Allowed values for Project.category and Project.status.
var (
	Categories = []string{"web", "desktop", "tool"}
	Statuses   = []string{"live", "open-source", "internal"}
)

// Mode selects create or update validation.
type Mode int

const (
	Create Mode = iota
	Update
)

// Text is an optional nullable text column: Set says whether the client sent the
// field at all, Val is nil for SQL NULL.
type Text struct {
	Set bool
	Val *string
}

// Input is the validated, partial project payload. A nil pointer / Set == false
// means "field not provided".
type Input struct {
	Title       *string
	Description *string
	Slug        Text
	ImageURL    Text
	LiveURL     Text
	GithubURL   Text
	ReleaseURL  Text
	Year        Text
	Role        Text
	Category    *string
	Status      *string
	Tags        *[]string
	Highlights  *[]string
	Featured    *bool
	Order       *int
	Visible     *bool
}

// jsTrim trims like String.prototype.trim (Unicode space plus the BOM).
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
}

// Slugify lowercases, collapses every run of characters outside [a-z0-9] into "-"
// and trims "-" from both ends.
func Slugify(text string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// UniqueSlug returns base if free, otherwise base-2, base-3, ... (first free counter).
func UniqueSlug(base string, taken []string) string {
	used := make(map[string]struct{}, len(taken))
	for _, s := range taken {
		used[s] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for n := 2; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ParseInput is the port of parseProjectInput. It returns a non-empty message when the
// body is invalid (always a 400).
//
// Lenient rules kept from Express:
//   - tags: applied only when the value is a JSON array. Express stored whatever array
//     it got (Prisma would then fail on non-string items with a 500); here non-string
//     items are dropped and the string items stored.
//   - order: applied only when a JSON number. Fractions are truncated toward zero
//     (Prisma would reject a float for an INTEGER column); numbers outside the int32
//     range are ignored like any other wrong value.
//   - visible: applied only when a JSON boolean.
func ParseInput(f httpx.Fields, mode Mode) (Input, string) {
	var in Input

	for _, key := range []string{"title", "description"} {
		if !f.Has(key) {
			if mode == Create {
				return in, "Judul dan deskripsi proyek wajib diisi."
			}
			continue
		}
		v, ok := f.String(key)
		if !ok || jsTrim(v) == "" {
			return in, `Field "` + key + `" harus berupa teks yang tidak kosong.`
		}
		v = jsTrim(v)
		if key == "title" {
			in.Title = &v
		} else {
			in.Description = &v
		}
	}

	nullable := []struct {
		key string
		dst *Text
	}{
		{"imageUrl", &in.ImageURL},
		{"liveUrl", &in.LiveURL},
		{"githubUrl", &in.GithubURL},
		{"releaseUrl", &in.ReleaseURL},
		{"year", &in.Year},
		{"role", &in.Role},
	}
	for _, n := range nullable {
		if !f.Has(n.key) {
			continue
		}
		if f.IsNull(n.key) {
			*n.dst = Text{Set: true}
			continue
		}
		v, ok := f.String(n.key)
		if !ok {
			return in, `Field "` + n.key + `" harus berupa teks atau null.`
		}
		v = jsTrim(v)
		if v == "" {
			*n.dst = Text{Set: true}
		} else {
			*n.dst = Text{Set: true, Val: &v}
		}
	}

	if f.Has("slug") {
		raw := ""
		if !f.IsNull("slug") {
			v, ok := f.String("slug")
			if !ok {
				return in, `Field "slug" harus berupa teks atau null.`
			}
			raw = jsTrim(v)
		}
		if raw == "" {
			if mode == Update {
				in.Slug = Text{Set: true}
			}
		} else {
			slug := Slugify(raw)
			if slug == "" {
				return in, `Field "slug" harus mengandung huruf atau angka.`
			}
			in.Slug = Text{Set: true, Val: &slug}
		}
	}

	if f.Has("category") {
		v, ok := f.String("category")
		if !ok || !contains(Categories, v) {
			return in, `Field "category" harus salah satu dari: ` + strings.Join(Categories, ", ") + `.`
		}
		in.Category = &v
	}

	if f.Has("status") {
		v, ok := f.String("status")
		if !ok || !contains(Statuses, v) {
			return in, `Field "status" harus salah satu dari: ` + strings.Join(Statuses, ", ") + `.`
		}
		in.Status = &v
	}

	if f.Has("highlights") {
		items, ok := f.StringSlice("highlights")
		if !ok {
			return in, `Field "highlights" harus berupa daftar teks.`
		}
		out := make([]string, 0, len(items))
		for _, h := range items {
			if h = jsTrim(h); h != "" {
				out = append(out, h)
			}
		}
		in.Highlights = &out
	}

	if f.Has("featured") {
		v, ok := f.Bool("featured")
		if !ok {
			return in, `Field "featured" harus true atau false.`
		}
		in.Featured = &v
	}

	if f.IsArray("tags") {
		out := []string{}
		var raw []any
		if err := json.Unmarshal(f["tags"], &raw); err == nil {
			for _, item := range raw {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
		}
		in.Tags = &out
	}
	if n, ok := f.Number("order"); ok && !math.IsNaN(n) {
		if t := math.Trunc(n); t >= math.MinInt32 && t <= math.MaxInt32 {
			v := int(t)
			in.Order = &v
		}
	}
	if v, ok := f.Bool("visible"); ok {
		in.Visible = &v
	}

	return in, ""
}
