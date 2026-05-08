package encoding

import (
	"crypto/rand"

	"github.com/shengdoushi/base58"
)

var (
	alphabet = base58.NewAlphabet("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")
)

const (
	idSizeBytes = 12
)

// NewRandomIdentifier generates a new, unique, cryptographically-secure
// identifier with the given prefix.
func NewRandomIdentifier(prefix string) string {
	bytes := make([]byte, idSizeBytes)
	_, _ = rand.Read(bytes)
	return prefix + "_" + base58.Encode(bytes, alphabet)
}

// NewRandomIdentifierWithLength is the same as NewRandomIdentifier except
// the caller may specify the number of bytes the random part of the string
// should be.
func NewRandomIdentifierWithLength(prefix string, length int) string {
	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)
	return prefix + "_" + base58.Encode(bytes, alphabet)
}

// NewRandomString generates a new cryptographically-secure random string
// with the given length
func NewRandomString(length int) string {
	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)
	return base58.Encode(bytes, alphabet)[0:length]
}
