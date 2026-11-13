package blog

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mrfroglab/portofolio-api-go/internal/httpx"
)

func fields(t *testing.T, js string) httpx.Fields {
	t.Helper()
	f := httpx.Fields{}
	if err := json.Unmarshal([]byte(js), &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestParseCreate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want Video
		msg  string
	}{
		{"minimal gets defaults", `{"videoId":"123"}`, Video{VideoID: "123", Caption: "", Tags: []string{}, Visible: true}, ""},
		{"missing videoId", `{}`, Video{}, MsgRequired},
		{"empty videoId", `{"videoId":""}`, Video{}, MsgRequired},
		{"null videoId", `{"videoId":null}`, Video{}, MsgRequired},
		{"numeric videoId", `{"videoId":123}`, Video{}, `Field "videoId" harus berupa teks.`},
		{"full", `{"videoId":"1","caption":"c","tags":["a","b"],"order":3,"visible":false}`, Video{VideoID: "1", Caption: "c", Tags: []string{"a", "b"}, Order: 3, Visible: false}, ""},
		{"falsy caption -> empty", `{"videoId":"1","caption":null}`, Video{VideoID: "1", Tags: []string{}, Visible: true}, ""},
		{"numeric caption", `{"videoId":"1","caption":5}`, Video{}, `Field "caption" harus berupa teks.`},
		{"non-array tags ignored", `{"videoId":"1","tags":"x"}`, Video{VideoID: "1", Tags: []string{}, Visible: true}, ""},
		{"non-string tag items", `{"videoId":"1","tags":[1]}`, Video{}, msgBadTags()},
		{"string order ignored", `{"videoId":"1","order":"2","visible":"x"}`, Video{VideoID: "1", Tags: []string{}, Visible: true}, ""},
		{"fractional order", `{"videoId":"1","order":1.5}`, Video{}, msgBadOrder()},
		{"negative order ok", `{"videoId":"1","order":-2}`, Video{VideoID: "1", Order: -2, Tags: []string{}, Visible: true}, ""},
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
	cur := Video{ID: 1, VideoID: "v", Caption: "c", Tags: []string{"x"}, Order: 2, Visible: true}
	cases := []struct {
		name string
		body string
		mut  func(*Video)
		msg  string
	}{
		{"empty keeps all", `{}`, func(*Video) {}, ""},
		{"videoId", `{"videoId":"n"}`, func(v *Video) { v.VideoID = "n" }, ""},
		{"empty caption applied", `{"caption":""}`, func(v *Video) { v.Caption = "" }, ""},
		{"empty tags array applied", `{"tags":[]}`, func(v *Video) { v.Tags = []string{} }, ""},
		{"non-array tags ignored", `{"tags":"a"}`, func(*Video) {}, ""},
		{"bad tag items", `{"tags":[1,2]}`, nil, msgBadTags()},
		{"order", `{"order":0}`, func(v *Video) { v.Order = 0 }, ""},
		{"visible", `{"visible":false}`, func(v *Video) { v.Visible = false }, ""},
		{"null videoId", `{"videoId":null}`, nil, `Field "videoId" tidak boleh null.`},
		{"null caption", `{"caption":null}`, nil, `Field "caption" tidak boleh null.`},
		{"numeric videoId", `{"videoId":1}`, nil, `Field "videoId" harus berupa teks.`},
		{"null order/visible keep existing", `{"order":null,"visible":null}`, func(*Video) {}, ""},
		{"fractional order", `{"order":0.1}`, nil, msgBadOrder()},
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
