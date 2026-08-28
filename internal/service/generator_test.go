package service

import "testing"

func TestGeneratorGenerate(t *testing.T) {
	generator := Generator{}
	first := generator.Generate("https://example.com/articles/go", 0)
	second := generator.Generate("https://example.com/articles/go", 0)
	withDifferentSalt := generator.Generate("https://example.com/articles/go", 1)

	if first != second {
		t.Errorf("Generate() must be deterministic: first = %q, second = %q", first, second)
	}
	if first == withDifferentSalt {
		t.Errorf("Generate() with different salt = %q, want a different code", withDifferentSalt)
	}
	if len(first) != codeLength {
		t.Errorf("Generate() length = %d, want %d", len(first), codeLength)
	}

	for _, character := range first {
		if !isBase62Character(character) {
			t.Errorf("Generate() returned non-base62 character %q in %q", character, first)
		}
	}
}

func TestEncodeBase62(t *testing.T) {
	tests := []struct {
		number uint64
		want   string
	}{
		{number: 0, want: "0000000"},
		{number: 61, want: "000000Z"},
		{number: 62, want: "0000010"},
	}

	for _, tt := range tests {
		if got := encodeBase62(tt.number); got != tt.want {
			t.Errorf("encodeBase62(%d) = %q, want %q", tt.number, got, tt.want)
		}
	}
}

func isBase62Character(character rune) bool {
	for _, allowed := range base62 {
		if character == allowed {
			return true
		}
	}

	return false
}
