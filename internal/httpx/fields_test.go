package httpx

import (
	"encoding/json"
	"testing"
)

func fieldsOf(t *testing.T, body string) Fields {
	t.Helper()
	f := Fields{}
	if err := json.Unmarshal([]byte(body), &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNullIsNotAValue(t *testing.T) {
	f := fieldsOf(t, `{"s":null,"b":null,"n":null,"a":null}`)
	if _, ok := f.String("s"); ok {
		t.Error("String(null) must not be ok")
	}
	if _, ok := f.Bool("b"); ok {
		t.Error("Bool(null) must not be ok")
	}
	if _, ok := f.Number("n"); ok {
		t.Error("Number(null) must not be ok")
	}
	if _, ok := f.Int("n"); ok {
		t.Error("Int(null) must not be ok")
	}
	if _, ok := f.StringSlice("a"); ok {
		t.Error("StringSlice(null) must not be ok")
	}
	if !f.IsNull("s") || !f.Has("s") || f.Truthy("s") {
		t.Error("null must be present, null and falsy")
	}
}

func TestTypedGetters(t *testing.T) {
	f := fieldsOf(t, `{"s":"x","b":false,"n":2.5,"i":3,"a":["p","q"],"mixed":["p",1],"zero":0,"empty":""}`)
	if v, ok := f.String("s"); !ok || v != "x" {
		t.Error("String")
	}
	if _, ok := f.String("n"); ok {
		t.Error("String must reject numbers")
	}
	if v, ok := f.Bool("b"); !ok || v {
		t.Error("Bool")
	}
	if _, ok := f.Int("n"); ok {
		t.Error("Int must reject 2.5")
	}
	if v, ok := f.Int("i"); !ok || v != 3 {
		t.Error("Int")
	}
	if _, ok := f.StringSlice("mixed"); ok || !f.IsArray("mixed") {
		t.Error("mixed array: not a string slice, but an array")
	}
	if f.Truthy("zero") || f.Truthy("empty") || f.Truthy("missing") || !f.Truthy("s") || !f.Truthy("a") {
		t.Error("Truthy mismatch with JS")
	}
}
