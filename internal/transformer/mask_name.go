package transformer

import (
	"crypto/sha256"
	"encoding/binary"
)

type maskName struct{}

func init() { Register(&maskName{}) }

func (m *maskName) Name() string { return "mask-name" }

func (m *maskName) Transform(value string, col ColumnInfo) (string, error) {
	h := sha256.Sum256([]byte(value))
	idx := binary.BigEndian.Uint32(h[:4])

	firstNames := []string{
		"James", "Maria", "John", "Ana", "Robert", "Linda",
		"David", "Sarah", "Michael", "Emma", "William", "Olivia",
		"Richard", "Sophia", "Thomas", "Isabella", "Daniel", "Mia",
	}
	lastNames := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia",
		"Miller", "Davis", "Rodriguez", "Martinez", "Wilson", "Anderson",
		"Taylor", "Thomas", "Moore", "Jackson", "Martin", "Lee",
	}

	first := firstNames[idx%uint32(len(firstNames))]
	last := lastNames[(idx/uint32(len(firstNames)))%uint32(len(lastNames))]
	return first + " " + last, nil
}
