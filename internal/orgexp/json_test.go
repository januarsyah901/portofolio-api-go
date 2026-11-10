package orgexp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCreatedAtJSONFormat(t *testing.T) {
	out, err := json.Marshal(Entry{CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 120_000_000, time.UTC)})
	if err != nil || strings.Count(string(out), "createdAt") != 1 || !strings.Contains(string(out), `"createdAt":"2026-01-02T03:04:05.120Z"`) {
		t.Fatalf("got %s, %v", out, err)
	}
}
