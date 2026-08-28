package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/omar-shahieen/url-shortner/internal/model"
)

func TestAliasValidatorValidate(t *testing.T) {
	validator := AliasValidator{}
	tooLong := strings.Repeat("a", maxAliasLength+1)

	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{name: "alphanumeric", alias: "myAlias123"},
		{name: "hyphen and underscore", alias: "my-alias_1"},
		{name: "case-sensitive reserved word", alias: "API"},
		{name: "too short", alias: "ab", wantErr: true},
		{name: "too long", alias: tooLong, wantErr: true},
		{name: "reserved api", alias: "api", wantErr: true},
		{name: "reserved stats", alias: "stats", wantErr: true},
		{name: "reserved health", alias: "health", wantErr: true},
		{name: "space", alias: "my alias", wantErr: true},
		{name: "unicode", alias: "café", wantErr: true},
		{name: "slash", alias: "my/alias", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.alias)
			if tt.wantErr {
				if !errors.Is(err, model.ErrInvalidAlias) {
					t.Errorf("Validate(%q) error = %v, want errors.Is(_, ErrInvalidAlias)", tt.alias, err)
				}
				return
			}
			if err != nil {
				t.Errorf("Validate(%q) error = %v", tt.alias, err)
			}
		})
	}
}
