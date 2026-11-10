package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunArgumentErrors(t *testing.T) {
	cases := [][]string{
		nil,
		{"create-user"},
		{"create-user", " "},
		{"nope", "bob"},
		{"create-user", "bob", "extra"},
	}
	for _, args := range cases {
		if err := run(args, strings.NewReader("pw\n"), &bytes.Buffer{}, "postgres://x"); err == nil {
			t.Errorf("run(%v) = nil, want error", args)
		}
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	err := run([]string{"create-user", "bob"}, strings.NewReader("secret\n"), &bytes.Buffer{}, "")
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadPassword(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{"hunter2hunter2\n", "hunter2hunter2", false},
		{"crlf-password\r\n", "crlf-password", false},
		{"no-newline", "no-newline", false},
		{"  spaces kept  \n", "  spaces kept  ", false},
		{"\n", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := readPassword(strings.NewReader(c.in), &bytes.Buffer{})
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("readPassword(%q) = %q, %v", c.in, got, err)
		}
	}
}
