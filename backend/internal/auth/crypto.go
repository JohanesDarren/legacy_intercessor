package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// randToken returns a URL-safe random string with n bytes of entropy.
func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("auth: crypto/rand unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken returns the SHA-256 hex digest of a token. Raw tokens are never
// stored, only their digests, so a database leak cannot be replayed.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
