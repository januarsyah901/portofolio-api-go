package orgexp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

func fields(t *testing.T, js string) httpx.Fields {
	t.Helper()
	f := httpx.Fields{}
	var anyVal any
	if err := json.Unmarshal([]byte(js), &anyVal); err != nil {
		t.Fatal(err)
	}
	if _, isObj := anyVal.(map[string]any); !isObj {
		return f // DecodeObject treats a non-object body as {}
	}
	if err := json.Unmarshal([]byte(js), &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// parse returns the parsed input overlaid on the create defaults (or the message).
func parse(t *testing.T, body string, mode Mode) (Entry, string) {
	t.Helper()
	in, msg := ParseInput(fields(t, body), mode)
	if msg != "" {
		return Entry{}, msg
	}
	return in.Apply(Defaults()), ""
}

func ptr[T any](v T) *T { return &v }

const valid = `"period":"2026","organization":"GSA","role":"Ambassador"`

func base() Entry {
	e := Defaults()
	e.Period, e.Organization, e.Role = "2026", "GSA", "Ambassador"
	return e
}

// Port of src/lib/orgExperience.test.ts "parseOrgExperienceInput (create)".
func TestParseCreate(t *testing.T) {
	t.Run("requires period, organization and role", func(t *testing.T) {
		for _, body := range []string{
			`{}`,
			`{"period":"2026","organization":"GSA"}`,
			`{"organization":"GSA","role":"R"}`,
			`{"period":"2026","role":"R"}`,
			`null`, // Express: non-object body is treated as {}
			`[]`,
			`"str"`,
		} {
			if _, msg := parse(t, body, Create); msg != MsgRequired {
				t.Errorf("%s: msg = %q, want %q", body, msg, MsgRequired)
			}
		}
	})

	t.Run("accepts a minimal body and trims text", func(t *testing.T) {
		got, msg := parse(t, `{"period":" 2026 ","organization":" GSA ","role":" Ambassador "}`, Create)
		if msg != "" || !reflect.DeepEqual(got, base()) {
			t.Fatalf("got %+v %q", got, msg)
		}
	})

	t.Run("rejects blank required text", func(t *testing.T) {
		_, msg := parse(t, `{"period":"2026","organization":"   ","role":"R"}`, Create)
		if msg != `Field "organization" harus berupa teks yang tidak kosong.` {
			t.Fatalf("msg = %q", msg)
		}
	})

	t.Run("validates optional fields", func(t *testing.T) {
		cases := map[string]string{
			`"type":"club"`:       `Field "type" harus salah satu dari: organization, event.`,
			`"type":5`:            `Field "type" harus salah satu dari: organization, event.`,
			`"type":null`:         `Field "type" harus salah satu dari: organization, event.`,
			`"highlights":"x"`:    `Field "highlights" harus berupa daftar teks.`,
			`"highlights":[1]`:    `Field "highlights" harus berupa daftar teks.`,
			`"highlights":null`:   `Field "highlights" harus berupa daftar teks.`,
			`"order":1.5`:         `Field "order" harus berupa bilangan bulat.`,
			`"order":"1"`:         `Field "order" harus berupa bilangan bulat.`,
			`"order":null`:        `Field "order" harus berupa bilangan bulat.`,
			`"order":99999999999`: `Field "order" harus berupa bilangan bulat.`,
			`"visible":"yes"`:     `Field "visible" harus true atau false.`,
			`"visible":null`:      `Field "visible" harus true atau false.`,
			`"logo":5`:            `Field "logo" harus berupa teks atau null.`,
			`"description":5`:     `Field "description" harus berupa teks.`,
			`"description":null`:  `Field "description" harus berupa teks.`,
			`"role":5`:            `Field "role" harus berupa teks yang tidak kosong.`,
		}
		for extra, want := range cases {
			if _, msg := parse(t, `{`+valid+`,`+extra+`}`, Create); msg != want {
				t.Errorf("%s: msg = %q, want %q", extra, msg, want)
			}
		}
	})

	t.Run("normalizes highlights and logo", func(t *testing.T) {
		got, msg := parse(t, `{`+valid+`,"type":"event","highlights":[" a ","","b"],"logo":"  ","order":2,"visible":false}`, Create)
		want := base()
		want.Type, want.Highlights, want.Logo, want.Order, want.Visible = "event", []string{"a", "b"}, nil, 2, false
		if msg != "" || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v %q", got, msg)
		}
	})

	t.Run("keeps a trimmed logo and integer-valued floats", func(t *testing.T) {
		got, _ := parse(t, `{`+valid+`,"logo":" /x.png ","order":3.0}`, Create)
		if got.Logo == nil || *got.Logo != "/x.png" || got.Order != 3 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("ignores unknown fields", func(t *testing.T) {
		got, msg := parse(t, `{`+valid+`,"id":99,"hacker":true}`, Create)
		if msg != "" || !reflect.DeepEqual(got, base()) {
			t.Fatalf("got %+v %q", got, msg)
		}
	})

	t.Run("trims JS whitespace including BOM", func(t *testing.T) {
		got, _ := parse(t, "{\"period\":\"\\uFEFF 2026\\u00a0\",\"organization\":\"o\",\"role\":\"r\"}", Create)
		if got.Period != "2026" {
			t.Fatalf("period = %q", got.Period)
		}
	})
}

// Port of "parseOrgExperienceInput (update)".
func TestParseUpdate(t *testing.T) {
	t.Run("allows an empty body and partial updates", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{}`), Update)
		if msg != "" || !reflect.DeepEqual(in, Input{}) {
			t.Fatalf("empty: %+v %q", in, msg)
		}
		in, msg = ParseInput(fields(t, `{"role":"Ketua"}`), Update)
		if msg != "" || in.Role == nil || *in.Role != "Ketua" || in.Period != nil || in.Organization != nil {
			t.Fatalf("partial: %+v %q", in, msg)
		}
	})

	t.Run("still rejects invalid values and blank required text", func(t *testing.T) {
		if _, msg := ParseInput(fields(t, `{"role":""}`), Update); msg == "" {
			t.Error("blank role accepted")
		}
		if _, msg := ParseInput(fields(t, `{"type":"nope"}`), Update); msg == "" {
			t.Error("bad type accepted")
		}
	})

	t.Run("can clear the logo with null", func(t *testing.T) {
		in, msg := ParseInput(fields(t, `{"logo":null}`), Update)
		if msg != "" || in.Logo == nil || *in.Logo != nil {
			t.Fatalf("got %+v %q", in, msg)
		}
		cur := base()
		cur.Logo = ptr("old")
		if got := in.Apply(cur); got.Logo != nil {
			t.Fatalf("logo not cleared: %+v", got)
		}
	})

	t.Run("apply only overwrites provided fields", func(t *testing.T) {
		cur := base()
		cur.Highlights, cur.Order = []string{"keep"}, 7
		in, _ := ParseInput(fields(t, `{"visible":false,"highlights":[]}`), Update)
		got := in.Apply(cur)
		if got.Visible || got.Order != 7 || got.Role != "Ambassador" || len(got.Highlights) != 0 || got.Highlights == nil {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestMessagesAreIndonesian(t *testing.T) {
	for _, m := range []string{MsgRequired, MsgBadID, MsgNotFound, MsgDeleted} {
		if strings.TrimSpace(m) == "" {
			t.Fatal("empty message")
		}
	}
}
