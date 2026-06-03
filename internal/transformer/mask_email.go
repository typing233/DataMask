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
