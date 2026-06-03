package transformer

import (
	"math/rand"
	"strconv"
)

type randomInt struct{}

func init() { Register(&randomInt{}) }

func (r *randomInt) Name() string { return "random-int" }

func (r *randomInt) Transform(value string, col ColumnInfo) (string, error) {
	n := rand.Intn(1000000)
	return strconv.Itoa(n), nil
}
