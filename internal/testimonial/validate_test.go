package testimonial

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/januarsyah901/portofolio-api-go/internal/httpx"
)

func fields(t *testing.T, js string) httpx.Fields {
	t.Helper()
	f := httpx.Fields{}
	if err := json.Unmarshal([]byte(js), &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func str(s string) *string { return &s }

func TestParseCreate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want Testimonial
		msg  string
	}{
		{"minimal gets defaults", `{"name":"A","role":"B","text":"C"}`, Testimonial{Name: "A", Role: "B", Text: "C", Rating: 5, Visible: true}, ""},
		{"missing name", `{"role":"B","text":"C"}`, Testimonial{}, MsgRequired},
		{"empty text is falsy", `{"name":"A","role":"B","text":""}`, Testimonial{}, MsgRequired},
		{"null role is falsy", `{"name":"A","role":null,"text":"C"}`, Testimonial{}, MsgRequired},
		{"zero name is falsy", `{"name":0,"role":"B","text":"C"}`, Testimonial{}, MsgRequired},
		{"non-string truthy name", `{"name":123,"role":"B","text":"C"}`, Testimonial{}, `Field "name" harus berupa teks.`},
		{"object text", `{"name":"A","role":"B","text":{}}`, Testimonial{}, `Field "text" harus berupa teks.`},
		{"empty company -> null", `{"name":"A","role":"B","text":"C","company":"","avatarUrl":null}`, Testimonial{Name: "A", Role: "B", Text: "C", Rating: 5, Visible: true}, ""},
		{"company and avatar kept", `{"name":"A","role":"B","text":"C","company":"X","avatarUrl":"u"}`, Testimonial{Name: "A", Role: "B", Text: "C", Company: str("X"), AvatarURL: str("u"), Rating: 5, Visible: true}, ""},
		{"non-string company", `{"name":"A","role":"B","text":"C","company":5}`, Testimonial{}, `Field "company" harus berupa teks.`},
		{"rating number", `{"name":"A","role":"B","text":"C","rating":3,"visible":false}`, Testimonial{Name: "A", Role: "B", Text: "C", Rating: 3, Visible: false}, ""},
		{"rating zero is a number", `{"name":"A","role":"B","text":"C","rating":0}`, Testimonial{Name: "A", Role: "B", Text: "C", Rating: 0, Visible: true}, ""},
		{"rating string ignored", `{"name":"A","role":"B","text":"C","rating":"4","visible":"no"}`, Testimonial{Name: "A", Role: "B", Text: "C", Rating: 5, Visible: true}, ""},
		{"null rating/visible use defaults", `{"name":"A","role":"B","text":"C","rating":null,"visible":null}`, Testimonial{Name: "A", Role: "B", Text: "C", Rating: 5, Visible: true}, ""},
		{"fractional rating", `{"name":"A","role":"B","text":"C","rating":4.5}`, Testimonial{}, msgBadRating()},
		{"huge rating", `{"name":"A","role":"B","text":"C","rating":1e12}`, Testimonial{}, msgBadRating()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, msg := ParseCreate(fields(t, c.body))
			if msg != c.msg {
				t.Fatalf("msg = %q, want %q", msg, c.msg)
			}
			if msg == "" && !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestApplyUpdate(t *testing.T) {
	cur := Testimonial{ID: 1, Name: "A", Role: "B", Company: str("Co"), AvatarURL: str("av"), Text: "T", Rating: 4, Visible: true}
	cases := []struct {
		name string
		body string
		mut  func(*Testimonial)
		msg  string
	}{
		{"empty body keeps all", `{}`, func(*Testimonial) {}, ""},
		{"overwrite name", `{"name":"Z"}`, func(x *Testimonial) { x.Name = "Z" }, ""},
		{"empty string is applied", `{"text":""}`, func(x *Testimonial) { x.Text = "" }, ""},
		{"null company clears", `{"company":null}`, func(x *Testimonial) { x.Company = nil }, ""},
		{"null avatar clears", `{"avatarUrl":null}`, func(x *Testimonial) { x.AvatarURL = nil }, ""},
		{"empty company string kept as empty", `{"company":""}`, func(x *Testimonial) { x.Company = str("") }, ""},
		{"null name rejected", `{"name":null}`, nil, `Field "name" tidak boleh null.`},
		{"null role rejected", `{"role":null}`, nil, `Field "role" tidak boleh null.`},
		{"null text rejected", `{"text":null}`, nil, `Field "text" tidak boleh null.`},
		{"numeric name rejected", `{"name":123}`, nil, `Field "name" harus berupa teks.`},
		{"numeric company rejected", `{"company":1}`, nil, `Field "company" harus berupa teks atau null.`},
		{"rating applied", `{"rating":1}`, func(x *Testimonial) { x.Rating = 1 }, ""},
		{"rating string ignored", `{"rating":"1","visible":"x"}`, func(*Testimonial) {}, ""},
		{"visible applied", `{"visible":false}`, func(x *Testimonial) { x.Visible = false }, ""},
		{"null rating/visible keep existing", `{"rating":null,"visible":null}`, func(*Testimonial) {}, ""},
		{"fractional rating", `{"rating":2.5}`, nil, msgBadRating()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, msg := ApplyUpdate(fields(t, c.body), cur)
			if msg != c.msg {
				t.Fatalf("msg = %q, want %q", msg, c.msg)
			}
			if msg != "" {
				return
			}
			want := cur
			c.mut(&want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}
