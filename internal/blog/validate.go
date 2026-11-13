package blog

import (
	"fmt"
	"math"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

// Messages (identical to the Express version unless noted).
const (
	MsgRequired = "ID Video TikTok wajib diisi."
	MsgBadID    = "ID entry tidak valid."
	MsgNotFound = "Entry video tidak ditemukan."
	MsgDeleted  = "Entry video berhasil dihapus."
)

// New messages: Express let Prisma fail with a 500 for these inputs.
func msgNotText(key string) string { return fmt.Sprintf(`Field "%s" harus berupa teks.`, key) }
func msgNotNull(key string) string { return fmt.Sprintf(`Field "%s" tidak boleh null.`, key) }
func msgBadTags() string           { return `Field "tags" harus berupa daftar teks.` }
func msgBadOrder() string          { return `Field "order" harus berupa bilangan bulat.` }

// tagsValue mirrors Array.isArray(tags): a non-array is "not present"; an array that
// holds non-strings would make Prisma fail, so it is rejected.
func tagsValue(f httpx.Fields) ([]string, bool, string) {
	if !f.IsArray("tags") {
		return nil, false, ""
	}
	tags, ok := f.StringSlice("tags")
	if !ok {
		return nil, false, msgBadTags()
	}
	return tags, true, ""
}

func orderValue(f httpx.Fields) (int, bool, string) {
	n, ok := numOf(f, "order")
	if !ok {
		return 0, false, ""
	}
	if n != math.Trunc(n) || n < math.MinInt32 || n > math.MaxInt32 {
		return 0, false, msgBadOrder()
	}
	return int(n), true, ""
}

// ParseCreate validates a create body and returns the row to insert (or an error message).
func ParseCreate(f httpx.Fields) (Video, string) {
	if !f.Truthy("videoId") {
		return Video{}, MsgRequired
	}
	v := Video{Tags: []string{}, Visible: true}
	s, ok := strOf(f, "videoId")
	if !ok {
		return Video{}, msgNotText("videoId")
	}
	v.VideoID = s
	if f.Truthy("caption") { // caption || ''
		c, ok := strOf(f, "caption")
		if !ok {
			return Video{}, msgNotText("caption")
		}
		v.Caption = c
	}
	if tags, present, msg := tagsValue(f); msg != "" {
		return Video{}, msg
	} else if present {
		v.Tags = tags
	}
	if n, present, msg := orderValue(f); msg != "" {
		return Video{}, msg
	} else if present {
		v.Order = n
	}
	if b, ok := boolOf(f, "visible"); ok {
		v.Visible = b
	}
	return v, ""
}

// ApplyUpdate merges a PUT body into the existing row. Explicit null for the NOT NULL
// columns videoId/caption is rejected with 400 (Express answered 500).
func ApplyUpdate(f httpx.Fields, cur Video) (Video, string) {
	for _, p := range []struct {
		key string
		dst *string
	}{{"videoId", &cur.VideoID}, {"caption", &cur.Caption}} {
		if !f.Has(p.key) {
			continue
		}
		if f.IsNull(p.key) {
			return Video{}, msgNotNull(p.key)
		}
		s, ok := strOf(f, p.key)
		if !ok {
			return Video{}, msgNotText(p.key)
		}
		*p.dst = s
	}
	if tags, present, msg := tagsValue(f); msg != "" {
		return Video{}, msg
	} else if present {
		cur.Tags = tags
	}
	if n, present, msg := orderValue(f); msg != "" {
		return Video{}, msg
	} else if present {
		cur.Order = n
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
