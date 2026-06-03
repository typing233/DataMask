package pipeline

import (
	"strings"

	"github.com/bojin/datamask/internal/transformer"
)

func TransformRow(line string, columns []string, columnTypes []string, tableName string, transformers []transformer.Transformer) (string, error) {
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
		val, err := t.Transform(fields[i], transformer.ColumnInfo{
			TableName:  tableName,
			ColumnName: colName,
			DataType:   colType,
			Position:   i,
		})
		if err != nil {
			return "", err
		}
		fields[i] = val
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
