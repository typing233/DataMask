package pipeline

import (
	"fmt"
	"strings"

	"github.com/bojin/datamask/internal/codec"
	"github.com/bojin/datamask/internal/transformer"
)

func TransformRow(line string, columns []string, columnTypes []string, tableName string, transformers []transformer.Transformer) (string, error) {
	return TransformRowTyped(line, columns, columnTypes, tableName, transformers, codec.NewPostgresRegistry())
}

func TransformRowTyped(line string, columns []string, columnTypes []string, tableName string, transformers []transformer.Transformer, registry *codec.Registry) (string, error) {
	fields := strings.Split(line, "\t")

	for i, t := range transformers {
		if t == nil || i >= len(fields) {
			continue
		}
		if fields[i] == "\\N" {
			continue
		}
		colName := ""
		if i < len(columns) {
			colName = columns[i]
		}
		colType := ""
		if i < len(columnTypes) {
			colType = columnTypes[i]
		}

		// Decode: verify the raw value is valid for this column type
		_, err := registry.Decode(fields[i], colType)
		if err != nil {
			return "", fmt.Errorf("column %q (type %s): decode error: %w", colName, colType, err)
		}

		// Transform: transformer works on the raw COPY text value
		val, err := t.Transform(fields[i], transformer.ColumnInfo{
			TableName:  tableName,
			ColumnName: colName,
			DataType:   colType,
			Position:   i,
		})
		if err != nil {
			return "", fmt.Errorf("column %q: transform error: %w", colName, err)
		}

		// Encode: verify the transformed output is valid for this column type
		encodedVal := &codec.Value{Native: val}
		encoded, err := registry.Encode(encodedVal, colType)
		if err != nil {
			return "", fmt.Errorf("column %q (type %s): encode error after transform: value %q is not valid for type: %w", colName, colType, val, err)
		}
		fields[i] = encoded
	}

	return strings.Join(fields, "\t"), nil
}

func BuildTransformers(columns []string, columnMapping map[string]string) ([]transformer.Transformer, error) {
	result := make([]transformer.Transformer, len(columns))
	for i, col := range columns {
		txName, ok := columnMapping[col]
		if !ok {
			continue
		}
		t, err := transformer.Get(txName)
		if err != nil {
			return nil, err
		}
		result[i] = t
	}
	return result, nil
}
