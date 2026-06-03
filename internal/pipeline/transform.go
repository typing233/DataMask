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

		col := transformer.ColumnInfo{
			TableName:  tableName,
			ColumnName: colName,
			DataType:   colType,
			Position:   i,
		}

		// Decode: convert raw COPY text to native Go type
		decoded, err := registry.Decode(fields[i], colType)
		if err != nil {
			return "", fmt.Errorf("column %q (type %s): decode error: %w", colName, colType, err)
		}

		var result *codec.Value

		// If transformer implements TypedTransformer, pass decoded native value
		if typed, ok := t.(transformer.TypedTransformer); ok {
			native, err := typed.TransformTyped(decoded.Native, col)
			if err != nil {
				return "", fmt.Errorf("column %q: typed transform error: %w", colName, err)
			}
			result = &codec.Value{Native: native}
		} else {
			// Fallback: pass raw string to Transform, then wrap result for encoding
			val, err := t.Transform(fields[i], col)
			if err != nil {
				return "", fmt.Errorf("column %q: transform error: %w", colName, err)
			}
			result = &codec.Value{Native: val}
		}

		// Encode: convert native value back to COPY text format
		encoded, err := registry.Encode(result, colType)
		if err != nil {
			return "", fmt.Errorf("column %q (type %s): encode error after transform: value %v is not valid for type: %w", colName, colType, result.Native, err)
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
