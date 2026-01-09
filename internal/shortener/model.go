package shortener

import (
	"time"

	"github.com/google/uuid"
)

type Link struct {
	ID             uuid.UUID
	OriginalURL    string
	Slug           string
	AccessCount    int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastAccessedAt *time.Time
	DeletedAt      *time.Time
}
