package service

import (
	"crypto/sha256"
	"crypto/subtle"
)

// TimingSafeEqual compares two strings in constant time relative to their
// content. Both sides are hashed first so the comparison length never leaks the
// real length of the secret (mirrors the original timingSafeStringEqual).
func TimingSafeEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
