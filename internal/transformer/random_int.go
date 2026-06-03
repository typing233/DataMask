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

func (r *randomInt) TransformTyped(value interface{}, col ColumnInfo) (interface{}, error) {
	return int64(rand.Intn(1000000)), nil
}

func (r *randomInt) Description() string {
	return "Replaces value with a random integer between 0 and 999999"
}

func (r *randomInt) DetailedHelp() string {
	return `Generates a uniformly distributed random integer in [0, 999999].
Non-deterministic: same input produces different outputs across runs.
Useful for numeric columns where uniqueness is not required.`
}

func (r *randomInt) SupportedTypes() []string {
	return []string{"integer", "bigint", "smallint", "numeric"}
}

func (r *randomInt) Examples() []Example {
	return []Example{
		{Input: "42", Output: "583721", DataType: "integer"},
		{Input: "99999", Output: "127403", DataType: "bigint"},
	}
}
