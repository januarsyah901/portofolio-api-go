package project

import (
	"time"
)

// Timestamp serialises like Prisma's DateTime: RFC 3339 in UTC with exactly three
// fractional digits ("2026-11-10T12:00:00.000Z").
type Timestamp struct{ time.Time }

// MarshalJSON implements json.Marshaler.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format("2006-01-02T15:04:05.000Z") + `"`), nil
}

// Project mirrors the Prisma Project model. There is no omitempty anywhere: nullable
// columns are pointers (serialised as null) and slices are never nil (serialised as []).
type Project struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    *string   `json:"imageUrl"`
	LiveURL     *string   `json:"liveUrl"`
	GithubURL   *string   `json:"githubUrl"`
	Tags        []string  `json:"tags"`
	Slug        *string   `json:"slug"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	Year        *string   `json:"year"`
	Role        *string   `json:"role"`
	Highlights  []string  `json:"highlights"`
	ReleaseURL  *string   `json:"releaseUrl"`
	Featured    bool      `json:"featured"`
	Order       int       `json:"order"`
	Visible     bool      `json:"visible"`
	CreatedAt   Timestamp `json:"createdAt"`
	UpdatedAt   Timestamp `json:"updatedAt"`
}
