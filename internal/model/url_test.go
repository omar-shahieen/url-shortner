package model

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestURLIsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Second)
	future := now.Add(time.Second)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{name: "no expiry", expiresAt: nil, want: false},
		{name: "past expiry", expiresAt: &past, want: true},
		{name: "future expiry", expiresAt: &future, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := URL{ExpiresAt: tt.expiresAt}
			if got := u.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSentinelErrorsAreDefined(t *testing.T) {
	errs := []error{
		ErrNotFound,
		ErrAliasTaken,
		ErrInvalidAlias,
		ErrCodeGenerationExhausted,
		ErrExpired,
		ErrInvalidURL,
	}

	for _, err := range errs {
		if err == nil {
			t.Fatal("sentinel error must not be nil")
		}
	}
}

func TestURLJSONRoundTrip(t *testing.T) {
	expiresAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	want := URL{
		Code:          "abc1234",
		OriginalURL:   "https://example.com/path",
		CreatedAt:     time.Date(2026, time.January, 1, 3, 4, 5, 0, time.UTC),
		ExpiresAt:     &expiresAt,
		ClickCount:    12,
		IsCustomAlias: true,
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got URL
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Code != want.Code || got.OriginalURL != want.OriginalURL ||
		!got.CreatedAt.Equal(want.CreatedAt) || got.ExpiresAt == nil ||
		!got.ExpiresAt.Equal(*want.ExpiresAt) || got.ClickCount != want.ClickCount ||
		got.IsCustomAlias != want.IsCustomAlias {
		t.Errorf("JSON round trip = %#v, want %#v", got, want)
	}
}

func TestURLValidate(t *testing.T) {
	tests := []struct {
		name    string
		url     URL
		want    error
		wantErr bool
	}{
		{name: "valid HTTPS URL", url: URL{OriginalURL: "https://example.com/path"}},
		{name: "missing URL", url: URL{}, want: ErrInvalidURL},
		{name: "relative URL", url: URL{OriginalURL: "/path"}, want: ErrInvalidURL},
		{name: "unsupported scheme", url: URL{OriginalURL: "ftp://example.com"}, want: ErrInvalidURL},
		{name: "negative click count", url: URL{OriginalURL: "http://example.com", ClickCount: -1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.url.Validate()
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Validate() error = %v, want errors.Is(_, %v)", err, tt.want)
				return
			}
			if tt.wantErr && err == nil {
				t.Error("Validate() error = nil, want an error")
			}
			if tt.want == nil && !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v", err)
			}
		})
	}
}
