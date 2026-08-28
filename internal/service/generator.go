// Package service contains application use cases and pure supporting logic.
package service

import (
	"crypto/sha256"
	"math/big"
	"strconv"
)

const (
	codeLength = 7
	base62     = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var codeSpace = new(big.Int).Exp(big.NewInt(int64(len(base62))), big.NewInt(codeLength), nil)

// Generator creates deterministic, fixed-length short codes from a URL and salt.
type Generator struct{}

// Generate hashes longURL and salt with SHA-256, then maps the result to a
// seven-character base62 code.
func (Generator) Generate(longURL string, salt uint64) string {
	sum := sha256.Sum256([]byte(longURL + strconv.FormatUint(salt, 10)))

	number := new(big.Int).SetBytes(sum[:])
	number.Mod(number, codeSpace)

	return encodeBase62(number.Uint64())
}

func encodeBase62(number uint64) string {
	code := make([]byte, codeLength)
	for i := codeLength - 1; i >= 0; i-- {
		code[i] = base62[number%uint64(len(base62))]
		number /= uint64(len(base62))
	}

	return string(code)
}
