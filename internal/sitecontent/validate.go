package sitecontent

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
)

// Keys lists the editable sections, in the order shown in the Express error message.
var Keys = []string{"experience", "skills", "about", "contact"}

// IsKey reports whether key is a known section.
func IsKey(key string) bool {
	for _, k := range Keys {
		if k == key {
			return true
		}
	}
	return false
}

// invalid is the panic payload used to abort validation with a message (port of the TS Invalid class).
type invalid string

func fail(format string, args ...any) { panic(invalid(fmt.Sprintf(format, args...))) }

// jsSpace is the JS \s / String.prototype.trim whitespace set (differs from Go's unicode.IsSpace).
const jsSpaceClass = `\t\n\x0b\x0c\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var (
	httpsRe = regexp.MustCompile(`(?i)^https://[^` + jsSpaceClass + `]+$`)
	emailRe = regexp.MustCompile(`^[^` + jsSpaceClass + `@]+@[^` + jsSpaceClass + `@]+\.[^` + jsSpaceClass + `@]+$`)
)

func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// utf16Len is JS String.length: UTF-16 code units, so an emoji counts as 2.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if l := utf16.RuneLen(r); l > 0 {
			n += l
		} else {
			n++
		}
	}
	return n
}

func obj(v any, path string) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		fail(`"%s" harus berupa objek.`, path)
	}
	return m
}

func text(v any, path string, max int) string {
	s, ok := v.(string)
	if !ok {
		fail(`"%s" harus berupa teks.`, path)
	}
	t := strings.TrimFunc(s, isJSSpace)
	if t == "" {
		fail(`"%s" tidak boleh kosong.`, path)
	}
	if utf16Len(t) > max {
		fail(`"%s" maksimal %d karakter.`, path, max)
	}
	return t
}

func list[T any](v any, path string, max, min int, each func(item any, itemPath string) T) []T {
	arr, ok := v.([]any)
	if !ok {
		fail(`"%s" harus berupa daftar.`, path)
	}
	if len(arr) < min {
		fail(`"%s" minimal %d item.`, path, min)
	}
	if len(arr) > max {
		fail(`"%s" maksimal %d item.`, path, max)
	}
	out := make([]T, 0, len(arr)) // never nil: serialises as []
	for i, item := range arr {
		out = append(out, each(item, fmt.Sprintf("%s[%d]", path, i)))
	}
	return out
}

// boolean: a missing key means false; present but non-boolean is an error. (JSON null is "present".)
func boolean(o map[string]any, key, path string) bool {
	v, present := o[key]
	if !present {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		fail(`"%s" harus true atau false.`, path)
	}
	return b
}

func httpsURL(v any, path string) string {
	u := text(v, path, 300)
	if !httpsRe.MatchString(u) {
		fail(`"%s" harus berupa URL https://.`, path)
	}
	return u
}

func email(v any, path string) string {
	e := text(v, path, 120)
	if !emailRe.MatchString(e) {
		fail(`"%s" bukan alamat email yang valid.`, path)
	}
	return e
}

// Output shapes. Field order here is the order of the TS validators' object literals.

type ExperienceItem struct {
	Period      string `json:"period"`
	Badge       string `json:"badge"`
	Accent      bool   `json:"accent"`
	Title       string `json:"title"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

type Experience struct {
	Eyebrow string           `json:"eyebrow"`
	Title   string           `json:"title"`
	Items   []ExperienceItem `json:"items"`
}

type Skill struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type SkillCategory struct {
	Label       string  `json:"label"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Skills      []Skill `json:"skills"`
}

type Skills struct {
	Eyebrow    string          `json:"eyebrow"`
	Title      string          `json:"title"`
	Categories []SkillCategory `json:"categories"`
}

type SummaryRow struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Accent bool   `json:"accent"`
}

type About struct {
	Eyebrow      string       `json:"eyebrow"`
	Name         string       `json:"name"`
	Paragraphs   []string     `json:"paragraphs"`
	SummaryTitle string       `json:"summaryTitle"`
	Summary      []SummaryRow `json:"summary"`
}

type Contact struct {
	CtaTitle    string `json:"ctaTitle"`
	CtaSubtitle string `json:"ctaSubtitle"`
	BrandDesc   string `json:"brandDesc"`
	Email       string `json:"email"`
	Location    string `json:"location"`
	GitHub      string `json:"github"`
	LinkedIn    string `json:"linkedin"`
	X           string `json:"x"`
	Instagram   string `json:"instagram"`
}

func validateExperience(raw any) any {
	o := obj(raw, "experience")
	var out Experience
	out.Eyebrow = text(o["eyebrow"], "eyebrow", 100)
	out.Title = text(o["title"], "title", 200)
	out.Items = list(o["items"], "items", 20, 0, func(item any, p string) ExperienceItem {
		i := obj(item, p)
		var it ExperienceItem
		it.Period = text(i["period"], p+".period", 60)
		it.Badge = text(i["badge"], p+".badge", 80)
		it.Accent = boolean(i, "accent", p+".accent")
		it.Title = text(i["title"], p+".title", 200)
		it.Role = text(i["role"], p+".role", 120)
		it.Description = text(i["description"], p+".description", 2000)
		return it
	})
	return out
}

func validateSkills(raw any) any {
	o := obj(raw, "skills")
	var out Skills
	out.Eyebrow = text(o["eyebrow"], "eyebrow", 100)
	out.Title = text(o["title"], "title", 200)
	out.Categories = list(o["categories"], "categories", 12, 0, func(cat any, p string) SkillCategory {
		c := obj(cat, p)
		var sc SkillCategory
		sc.Label = text(c["label"], p+".label", 60)
		sc.Title = text(c["title"], p+".title", 120)
		sc.Description = text(c["description"], p+".description", 500)
		sc.Skills = list(c["skills"], p+".skills", 12, 0, func(skill any, sp string) Skill {
			s := obj(skill, sp)
			lv, ok := s["level"].(float64)
			if !ok || lv != float64(int64(lv)) || lv < 0 || lv > 100 {
				fail(`"%s.level" harus bilangan bulat 0-100.`, sp)
			}
			return Skill{Name: text(s["name"], sp+".name", 80), Level: int(lv)}
		})
		return sc
	})
	return out
}

func validateAbout(raw any) any {
	o := obj(raw, "about")
	var out About
	out.Eyebrow = text(o["eyebrow"], "eyebrow", 100)
	out.Name = text(o["name"], "name", 120)
	out.Paragraphs = list(o["paragraphs"], "paragraphs", 10, 1, func(p any, path string) string {
		return text(p, path, 2000)
	})
	out.SummaryTitle = text(o["summaryTitle"], "summaryTitle", 100)
	out.Summary = list(o["summary"], "summary", 12, 0, func(row any, p string) SummaryRow {
		r := obj(row, p)
		var sr SummaryRow
		sr.Label = text(r["label"], p+".label", 80)
		sr.Value = text(r["value"], p+".value", 200)
		sr.Accent = boolean(r, "accent", p+".accent")
		return sr
	})
	return out
}

func validateContact(raw any) any {
	o := obj(raw, "contact")
	var out Contact
	out.CtaTitle = text(o["ctaTitle"], "ctaTitle", 120)
	out.CtaSubtitle = text(o["ctaSubtitle"], "ctaSubtitle", 300)
	out.BrandDesc = text(o["brandDesc"], "brandDesc", 500)
	out.Email = email(o["email"], "email")
	out.Location = text(o["location"], "location", 120)
	out.GitHub = httpsURL(o["github"], "github")
	out.LinkedIn = httpsURL(o["linkedin"], "linkedin")
	out.X = httpsURL(o["x"], "x")
	out.Instagram = httpsURL(o["instagram"], "instagram")
	return out
}

var validators = map[string]func(any) any{
	"experience": validateExperience,
	"skills":     validateSkills,
	"about":      validateAbout,
	"contact":    validateContact,
}

// Validate checks and normalises raw (a value decoded from JSON into any) for key.
// On failure it returns the Indonesian message; unknown fields are dropped from the result.
func Validate(key string, raw any) (value any, msg string, ok bool) {
	v, known := validators[key]
	if !known {
		return nil, fmt.Sprintf("Section %q tidak dikenal.", key), false
	}
	defer func() {
		if p := recover(); p != nil {
			if inv, isInv := p.(invalid); isInv {
				value, msg, ok = nil, string(inv), false
				return
			}
			panic(p)
		}
	}()
	return v(raw), "", true
}
