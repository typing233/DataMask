package transformer

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type maskEmail struct{}

func init() { Register(&maskEmail{}) }

func (m *maskEmail) Name() string { return "mask-email" }

func (m *maskEmail) Transform(value string, col ColumnInfo) (string, error) {
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 {
		return "masked@example.com", nil
	}
	h := sha256.Sum256([]byte(parts[0]))
	masked := hex.EncodeToString(h[:])[:8]
	return masked + "@" + parts[1], nil
}

func (m *maskEmail) Description() string {
	return "Deterministically masks email local part using SHA-256, preserves domain"
}

func (m *maskEmail) DetailedHelp() string {
	return `Replaces the local part of an email address with a deterministic 8-character
hex string derived from SHA-256 hashing. The domain is preserved unchanged.
Non-email values are replaced with masked@example.com.
Deterministic: same input always produces the same output.`
}

func (m *maskEmail) SupportedTypes() []string {
	return []string{"text", "varchar", "character varying"}
}

func (m *maskEmail) Examples() []Example {
	return []Example{
		{Input: "alice@company.com", Output: "2bd806c9@company.com", DataType: "text"},
		{Input: "bob.smith@gmail.com", Output: "a6b54c20@gmail.com", DataType: "varchar"},
	}
}
