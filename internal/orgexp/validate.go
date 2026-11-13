package orgexp

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Types is the set of allowed values for "type".
var Types = []string{"organization", "event"}

// Messages (identical to the Express version unless noted).
const (
	MsgRequired = "Periode, nama organisasi/event, dan peran wajib diisi."
	MsgBadID    = "ID tidak valid."
	MsgNotFound = "Entri organisasi/event tidak ditemukan."
	MsgDeleted  = "Entri organisasi/event berhasil dihapus."
)

// Mode selects create or update validation.
type Mode int

// Validation modes.
const (
	Create Mode = iota
	Update
)

// Input is the validated subset of fields; nil means "not provided".
// Logo is a double pointer: nil = not provided, non-nil pointing to nil = set NULL.
type Input struct {
	Period       *string
	Organization *string
	Role         *string
	Description  *string
	Highlights   *[]string
	Type         *string
	Logo         **string
	Order        *int
	Visible      *bool
}

// jsTrim is String.prototype.trim: whitespace plus BOM.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r == '\uFEFF' || unicode.IsSpace(r) })
}

// ParseInput ports parseOrgExperienceInput. It returns an error message ("" when ok).
// Unknown fields are ignored and strings are trimmed.
func ParseInput(f httpx.Fields, mode Mode) (Input, string) {
	var in Input
	required := []struct {
		key string
		dst **string
	}{{"period", &in.Period}, {"organization", &in.Organization}, {"role", &in.Role}}
	for _, p := range required {
		if !f.Has(p.key) {
			if mode == Create {
				return Input{}, MsgRequired
			}
			continue
		}
		s, ok := strOf(f, p.key)
		if !ok || jsTrim(s) == "" {
			return Input{}, fmt.Sprintf(`Field "%s" harus berupa teks yang tidak kosong.`, p.key)
		}
		s = jsTrim(s)
		*p.dst = &s
	}

	if f.Has("description") {
		s, ok := strOf(f, "description")
		if !ok {
			return Input{}, `Field "description" harus berupa teks.`
		}
		s = jsTrim(s)
		in.Description = &s
	}

	if f.Has("highlights") {
		hs, ok := f.StringSlice("highlights")
		if !ok {
			return Input{}, `Field "highlights" harus berupa daftar teks.`
		}
		out := []string{}
		for _, h := range hs {
			if h = jsTrim(h); h != "" {
				out = append(out, h)
			}
		}
		in.Highlights = &out
	}

	if f.Has("type") {
		s, ok := strOf(f, "type")
		if !ok || !slices.Contains(Types, s) {
			return Input{}, fmt.Sprintf(`Field "type" harus salah satu dari: %s.`, strings.Join(Types, ", "))
		}
		in.Type = &s
	}

	if f.Has("logo") {
		if f.IsNull("logo") {
			var none *string
			in.Logo = &none
		} else {
			s, ok := strOf(f, "logo")
			if !ok {
				return Input{}, `Field "logo" harus berupa teks atau null.`
			}
			var v *string
			if s = jsTrim(s); s != "" {
				v = &s
			}
			in.Logo = &v
		}
	}

	if f.Has("order") {
		// Beyond int32 Express would crash inside Prisma (500); we report 400 instead.
		n, ok := numOf(f, "order")
		if !ok || n != math.Trunc(n) || n > math.MaxInt32 || n < math.MinInt32 {
			return Input{}, `Field "order" harus berupa bilangan bulat.`
		}
		o := int(n)
		in.Order = &o
	}

	if f.Has("visible") {
		b, ok := boolOf(f, "visible")
		if !ok {
			return Input{}, `Field "visible" harus true atau false.`
		}
		in.Visible = &b
	}
	return in, ""
}

// Apply overlays the provided fields onto an entry.
func (in Input) Apply(e Entry) Entry {
	if in.Period != nil {
		e.Period = *in.Period
	}
	if in.Organization != nil {
		e.Organization = *in.Organization
	}
	if in.Role != nil {
		e.Role = *in.Role
	}
	if in.Description != nil {
		e.Description = *in.Description
	}
	if in.Highlights != nil {
		e.Highlights = *in.Highlights
	}
	if in.Type != nil {
		e.Type = *in.Type
	}
	if in.Logo != nil {
		e.Logo = *in.Logo
	}
	if in.Order != nil {
		e.Order = *in.Order
	}
	if in.Visible != nil {
		e.Visible = *in.Visible
	}
	return e
}

// Defaults are the column defaults used for create.
func Defaults() Entry {
	return Entry{Type: "organization", Highlights: []string{}, Visible: true}
}

// httpx.Fields.String/Bool/Number report ok for JSON null (json.Unmarshal of null is a
// no-op), but JS typeof null is "object". These wrappers restore the typeof semantics.
func strOf(f httpx.Fields, key string) (string, bool) {
	if f.IsNull(key) {
		return "", false
	}
	return f.String(key)
}

func boolOf(f httpx.Fields, key string) (bool, bool) {
	if f.IsNull(key) {
		return false, false
	}
	return f.Bool(key)
}

func numOf(f httpx.Fields, key string) (float64, bool) {
	if f.IsNull(key) {
		return 0, false
	}
	return f.Number(key)
}
