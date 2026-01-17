package strutils

import (
	"crypto/rand"
	"errors"
	"ismartcoding/plainnas/internal/pkg/log"
	"strconv"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
)

func Chunk(s string, chunkSize int) []string {
	if chunkSize >= len(s) {
		return []string{s}
	}
	var chunks []string
	chunk := make([]rune, chunkSize)
	len := 0
	for _, r := range s {
		chunk[len] = r
		len++
		if len == chunkSize {
			chunks = append(chunks, string(chunk))
			len = 0
		}
	}
	if len > 0 {
		chunks = append(chunks, string(chunk[:len]))
	}
	return chunks
}

func ChaCha20Encrypt(key []byte, text []byte) []byte {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		log.Error(err)
		return nil
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		log.Error(err)
		return nil
	}
	return aead.Seal(nonce, nonce, text, nil)
}

func ChaCha20Decrypt(key []byte, ciphertext []byte) []byte {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		log.Error(err)
		return nil
	}
	if len(ciphertext) < aead.NonceSize() {
		log.Error(errors.New("ciphertext too short"))
		return nil
	}
	nonce, ciphertext := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Error(err)
		return nil
	}
	return plaintext
}

func ContainsI(a string, b string) bool {
	return strings.Contains(
		strings.ToLower(a),
		strings.ToLower(b),
	)
}

func ReverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// ParseInt parses a string to int, returning 0 on error or empty.
func ParseInt(s string) int {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// ParseIntDefault parses a string to int, returning def on error or empty.
func ParseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
