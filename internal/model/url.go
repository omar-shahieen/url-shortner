package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrNotFound indicates that a shortened URL does not exist.
	ErrNotFound = errors.New("url not found")
	// ErrAliasTaken indicates that a requested custom alias is already in use.
	ErrAliasTaken = errors.New("alias already taken")
	// ErrInvalidAlias indicates that a custom alias fails validation.
	ErrInvalidAlias = errors.New("invalid alias")
	// ErrCodeGenerationExhausted indicates that no unique generated code was found.
	ErrCodeGenerationExhausted = errors.New("code generation exhausted")
	// ErrExpired indicates that a shortened URL has passed its expiry time.
	ErrExpired = errors.New("url expired")
	// ErrInvalidURL indicates that an original URL is not a valid HTTP(S) URL.
	ErrInvalidURL = errors.New("invalid original URL")
)

// URL is a shortened URL and its associated metadata.
type URL struct {
	Code          string     `json:"code"`
	OriginalURL   string     `json:"originalUrl"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	ClickCount    int64      `json:"clickCount"`
	IsCustomAlias bool       `json:"isCustomAlias"`
}

// IsExpired reports whether the URL has reached its optional expiry time.
func (u URL) IsExpired() bool {
	return u.ExpiresAt != nil && !u.ExpiresAt.After(time.Now())
}

// Validate reports whether URL contains a valid destination and consistent metadata.
func (u URL) Validate() error {
	if strings.TrimSpace(u.OriginalURL) == "" {
		return fmt.Errorf("%w: URL is required", ErrInvalidURL)
	}

	parsed, err := url.ParseRequestURI(u.OriginalURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%w: must be an absolute HTTP(S) URL", ErrInvalidURL)
	}

	if u.ClickCount < 0 {
		return fmt.Errorf("click count cannot be negative")
	}

	return nil
}
