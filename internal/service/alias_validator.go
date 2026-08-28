package service

import (
	"fmt"

	"github.com/omar-shahieen/url-shortner/internal/model"
)

const (
	minAliasLength = 3
	maxAliasLength = 32
)

var reservedAliases = map[string]struct{}{
	"api":    {},
	"stats":  {},
	"health": {},
}

// AliasValidator validates user-supplied custom aliases.
type AliasValidator struct{}

// Validate reports whether alias is an allowed custom alias. Validation is
// case-sensitive: "api" is reserved, while "API" is allowed.
func (AliasValidator) Validate(alias string) error {
	if len(alias) < minAliasLength || len(alias) > maxAliasLength {
		return fmt.Errorf("%w: must be between %d and %d characters", model.ErrInvalidAlias, minAliasLength, maxAliasLength)
	}

	if _, reserved := reservedAliases[alias]; reserved {
		return fmt.Errorf("%w: reserved alias", model.ErrInvalidAlias)
	}

	for _, character := range alias {
		if !isAliasCharacter(character) {
			return fmt.Errorf("%w: contains unsupported character", model.ErrInvalidAlias)
		}
	}

	return nil
}

func isAliasCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '-' || character == '_'
}
