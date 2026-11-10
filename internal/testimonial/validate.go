package testimonial

import (
	"fmt"
	"math"

	"github.com/januarsyah901/portofolio-api-go/internal/httpx"
)

// Messages (identical to the Express version unless noted).
const (
	MsgRequired = "Nama, peran (role), dan pesan testimonial wajib diisi."
	MsgBadID    = "ID testimonial tidak valid."
	MsgNotFound = "Testimonial tidak ditemukan."
	MsgDeleted  = "Testimonial berhasil dihapus."
)

// The following messages are new: Express let Prisma fail with a 500 for these inputs.
func msgNotText(key string) string { return fmt.Sprintf(`Field "%s" harus berupa teks.`, key) }
func msgNotTextNull(key string) string {
	return fmt.Sprintf(`Field "%s" harus berupa teks atau null.`, key)
}
func msgNotNull(key string) string { return fmt.Sprintf(`Field "%s" tidak boleh null.`, key) }
func msgBadRating() string         { return `Field "rating" harus berupa bilangan bulat.` }

// ratingValue returns (value, present, errMsg) for a JSON-number rating. Non-numbers are
// "not present" (Express ignored them); numbers Prisma cannot store as Int are rejected.
func ratingValue(f httpx.Fields) (int, bool, string) {
	n, ok := numOf(f, "rating")
	if !ok {
		return 0, false, ""
	}
	if n != math.Trunc(n) || n < math.MinInt32 || n > math.MaxInt32 {
		return 0, false, msgBadRating()
	}
	return int(n), true, ""
}

// ParseCreate validates a create body and returns the row to insert (or an error message).
func ParseCreate(f httpx.Fields) (Testimonial, string) {
	if !f.Truthy("name") || !f.Truthy("role") || !f.Truthy("text") {
		return Testimonial{}, MsgRequired
	}
	t := Testimonial{Rating: 5, Visible: true}
	for _, p := range []struct {
		key string
		dst *string
	}{{"name", &t.Name}, {"role", &t.Role}, {"text", &t.Text}} {
		s, ok := strOf(f, p.key)
		if !ok {
			return Testimonial{}, msgNotText(p.key)
		}
		*p.dst = s
	}
	for _, p := range []struct {
		key string
		dst **string
	}{{"company", &t.Company}, {"avatarUrl", &t.AvatarURL}} {
		if !f.Truthy(p.key) { // falsy -> null
			continue
		}
		s, ok := strOf(f, p.key)
		if !ok {
			return Testimonial{}, msgNotText(p.key)
		}
		*p.dst = &s
	}
	if v, present, msg := ratingValue(f); msg != "" {
		return Testimonial{}, msg
	} else if present {
		t.Rating = v
	}
	if b, ok := boolOf(f, "visible"); ok {
		t.Visible = b
	}
	return t, ""
}

// ApplyUpdate merges a PUT body into the existing row ("field !== undefined ? field : existing").
// An explicit null is applied to the nullable columns (company, avatarUrl) and rejected with 400
// for NOT NULL columns, where Express would have crashed with a 500.
func ApplyUpdate(f httpx.Fields, cur Testimonial) (Testimonial, string) {
	for _, p := range []struct {
		key string
		dst *string
	}{{"name", &cur.Name}, {"role", &cur.Role}, {"text", &cur.Text}} {
		if !f.Has(p.key) {
			continue
		}
		if f.IsNull(p.key) {
			return Testimonial{}, msgNotNull(p.key)
		}
		s, ok := strOf(f, p.key)
		if !ok {
			return Testimonial{}, msgNotText(p.key)
		}
		*p.dst = s
	}
	for _, p := range []struct {
		key string
		dst **string
	}{{"company", &cur.Company}, {"avatarUrl", &cur.AvatarURL}} {
		if !f.Has(p.key) {
			continue
		}
		if f.IsNull(p.key) {
			*p.dst = nil
			continue
		}
		s, ok := strOf(f, p.key)
		if !ok {
			return Testimonial{}, msgNotTextNull(p.key)
		}
		*p.dst = &s
	}
	if v, present, msg := ratingValue(f); msg != "" {
		return Testimonial{}, msg
	} else if present {
		cur.Rating = v
	}
	if b, ok := boolOf(f, "visible"); ok {
		cur.Visible = b
	}
	return cur, ""
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
