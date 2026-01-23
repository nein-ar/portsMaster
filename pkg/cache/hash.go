package cache

import (
	"encoding/hex"
	"io"
	"os"

	"lukechampine.com/blake3"
)

// HashString returns the BLAKE3 hash of a string.
func HashString(s string) string {
	h := blake3.New(32, nil)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// HashFile returns the BLAKE3 hash of a file's content.
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

// HashBytes returns the BLAKE3 hash of a byte slice.
func HashBytes(b []byte) string {
	h := blake3.New(32, nil)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// Hasher helps accumulate hashes for complex objects.
type Hasher struct {
	h *blake3.Hasher
}

func NewHasher() *Hasher {
	return &Hasher{h: blake3.New(32, nil)}
}

func (h *Hasher) Add(s string) {
	h.h.Write([]byte(s))
}

func (h *Hasher) AddBytes(b []byte) {
	h.h.Write(b)
}

func (h *Hasher) Sum() string {
	return hex.EncodeToString(h.h.Sum(nil))
}
