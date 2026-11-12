package cache

import (
	"encoding/hex"
	"io"
	"os"

	"lukechampine.com/blake3"
)

// String hashing
func HashString(s string) string {
	h := blake3.New(32, nil)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// File hashing
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := blake3.New(32, nil)
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Byte hashing
func HashBytes(b []byte) string {
	h := blake3.New(32, nil)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
