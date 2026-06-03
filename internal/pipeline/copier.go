package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/klauspost/pgzip"
)

type CopyToFileParams struct {
	DSN         string
	Schema      string
	Table       string
	Columns     []string
	ColumnTypes []string
	OutputDir   string
	Transforms  map[string]string
}

type CopyResult struct {
	RowCount       int64
	DataFile       string
	CompressedSize int64
	OriginalSize   int64
}

func CopyTableToFile(ctx context.Context, params CopyToFileParams) (*CopyResult, error) {
	conn, err := pgx.Connect(ctx, params.DSN)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close(ctx)

	dataFileName := fmt.Sprintf("%s.%s.csv.gz", params.Schema, params.Table)
	dataFilePath := filepath.Join(params.OutputDir, dataFileName)

	f, err := os.Create(dataFilePath)
	if err != nil {
		return nil, fmt.Errorf("creating data file: %w", err)
	}
	defer f.Close()

	gzw, err := pgzip.NewWriterLevel(f, pgzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("creating gzip writer: %w", err)
	}
	defer gzw.Close()

	transformers, err := BuildTransformers(params.Columns, params.Transforms)
	if err != nil {
		return nil, fmt.Errorf("building transformers: %w", err)
	}

	qualifiedTable := fmt.Sprintf("%q.%q", params.Schema, params.Table)
	copySQL := fmt.Sprintf("COPY %s TO STDOUT", qualifiedTable)

	pr, pw := io.Pipe()

	copyErrCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		rawConn := conn.PgConn()
		_, err := rawConn.CopyTo(ctx, pw, copySQL)
		copyErrCh <- err
	}()

	var rowCount int64
	var originalSize int64
	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		transformed, err := TransformRow(line, params.Columns, params.ColumnTypes, params.Table, transformers)
		if err != nil {
			pr.CloseWithError(err)
			return nil, fmt.Errorf("transforming row: %w", err)
		}

		data := []byte(transformed + "\n")
		originalSize += int64(len(data))
		if _, err := gzw.Write(data); err != nil {
			return nil, fmt.Errorf("writing compressed data: %w", err)
		}
		rowCount++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning COPY output: %w", err)
	}

	if copyErr := <-copyErrCh; copyErr != nil {
		return nil, fmt.Errorf("COPY TO: %w", copyErr)
	}

	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("closing file: %w", err)
	}

	stat, _ := os.Stat(dataFilePath)
	compressedSize := int64(0)
	if stat != nil {
		compressedSize = stat.Size()
	}

	return &CopyResult{
		RowCount:       rowCount,
		DataFile:       dataFileName,
		CompressedSize: compressedSize,
		OriginalSize:   originalSize,
	}, nil
}
