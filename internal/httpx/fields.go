package httpx

import (
	"encoding/json"
)

// Fields wraps a decoded JSON object and answers "what JSON type is this field?",
// which is how the Express validators decided what to accept.
type Fields map[string]json.RawMessage

// Has reports whether the key is present (including an explicit null).
func (f Fields) Has(key string) bool {
	_, ok := f[key]
	return ok
}

// IsNull reports whether the key is present and is JSON null.
func (f Fields) IsNull(key string) bool {
	raw, ok := f[key]
	return ok && string(raw) == "null"
}

// value returns the raw JSON for key, treating a missing key and JSON null alike.
// json.Unmarshal of null into a string/bool/number is a silent no-op, so without this
// guard `"visible": null` would read as a valid false.
func (f Fields) value(key string) (json.RawMessage, bool) {
	raw, ok := f[key]
	if !ok || string(raw) == "null" {
		return nil, false
	}
	return raw, true
}

// String returns the value when it is a JSON string.
func (f Fields) String(key string) (string, bool) {
	raw, ok := f.value(key)
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// Bool returns the value when it is a JSON boolean.
func (f Fields) Bool(key string) (bool, bool) {
	raw, ok := f.value(key)
	if !ok {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, false
	}
	return b, true
}

// Number returns the value when it is a JSON number (like typeof x === 'number').
func (f Fields) Number(key string) (float64, bool) {
	raw, ok := f.value(key)
	if !ok {
		return 0, false
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false
	}
	return n, true
}

// Int returns the value when it is a JSON number with no fractional part
// (like Number.isInteger(x)).
func (f Fields) Int(key string) (int, bool) {
	n, ok := f.Number(key)
	if !ok || n != float64(int(n)) {
		return 0, false
	}
	return int(n), true
}

// StringSlice returns the value when it is a JSON array whose items are all strings.
func (f Fields) StringSlice(key string) ([]string, bool) {
	raw, ok := f.value(key)
	if !ok {
		return nil, false
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return nil, false
	}
	return out, true
}

// IsArray reports whether the value is a JSON array of any item type.
func (f Fields) IsArray(key string) bool {
	raw, ok := f[key]
	if !ok {
		return false
	}
	var arr []json.RawMessage
	return json.Unmarshal(raw, &arr) == nil && arr != nil
}

// Truthy mirrors JavaScript truthiness for a field (missing, null, false, 0, "" are falsy).
func (f Fields) Truthy(key string) bool {
	raw, ok := f[key]
	if !ok {
		return false
	}
	switch string(raw) {
	case "null", "false", "0", `""`:
		return false
	}
	if n, isNum := f.Number(key); isNum {
		return n != 0
	}
	return true
}
