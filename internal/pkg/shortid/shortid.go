package shortid

import (
	"math/big"
	"strings"

	"github.com/google/uuid"
)

const (
	// Removed ambiguous characters: 0, O, 1, I, l
	alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

// New generates a new short UUID
func New() string {
	return encode(uuid.New())
}

// encode converts UUID to base57 string
func encode(u uuid.UUID) string {
	// Convert UUID to big integer
	num := new(big.Int).SetBytes(u[:])

	// Calculate base57
	base := big.NewInt(int64(len(alphabet)))
	zero := big.NewInt(0)
	mod := new(big.Int)

	// Build encoded string
	var encoded strings.Builder

	// Continue encoding while number is not zero
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		encoded.WriteByte(alphabet[mod.Int64()])
	}

	// Reverse the string
	result := reverse(encoded.String())

	// Pad to 22 characters
	if len(result) < 22 {
		result = strings.Repeat(string(alphabet[0]), 22-len(result)) + result
	}

	return result
}

// reverse reverses a string
func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
