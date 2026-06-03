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

func (h *hashSHA256) Description() string {
	return "Produces full SHA-256 hex digest of the input value"
}

func (h *hashSHA256) DetailedHelp() string {
	return `Computes the SHA-256 hash of the raw input string and returns the full
64-character hexadecimal digest. Deterministic and one-way.
Useful for creating consistent pseudonymized identifiers.`
}

func (h *hashSHA256) SupportedTypes() []string {
	return []string{"text", "varchar", "character varying", "uuid", "bytea"}
}

func (h *hashSHA256) Examples() []Example {
	return []Example{
		{Input: "hello", Output: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", DataType: "text"},
	}
}
