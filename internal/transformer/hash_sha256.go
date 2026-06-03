package transformer

import (
	"crypto/sha256"
	"encoding/hex"
)

type hashSHA256 struct{}

func init() { Register(&hashSHA256{}) }

func (h *hashSHA256) Name() string { return "hash-sha256" }

func (h *hashSHA256) Transform(value string, col ColumnInfo) (string, error) {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:]), nil
}
