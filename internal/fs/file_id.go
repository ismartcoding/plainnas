package fs

import (
	"encoding/base64"
	"errors"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/strutils"
)

var (
	ErrInvalidFileID = errors.New("invalid file id")
	ErrForbidden     = errors.New("file is expired or does not exist")
)

// PathFromFileID decrypts the encrypted file id used by the public /fs endpoint.
// The id is base64.StdEncoding of ChaCha20-encrypted file path (keyed by URL token).
func PathFromFileID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrInvalidFileID
	}

	// Some URL parsers turn '+' into ' ' in query strings.
	id = strings.ReplaceAll(id, " ", "+")

	ciphertext, err := base64.StdEncoding.DecodeString(id)
	if err != nil {
		return "", ErrForbidden
	}

	token := db.GetURLToken()
	if token == "" {
		return "", ErrForbidden
	}
	key, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", ErrForbidden
	}

	plain := strutils.ChaCha20Decrypt(key, ciphertext)
	if plain == nil || len(plain) == 0 {
		return "", ErrForbidden
	}
	return string(plain), nil
}
