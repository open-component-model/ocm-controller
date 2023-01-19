package controllers

import (
	"compress/gzip"
	"fmt"
	"io"
)

func Ungzip(in io.Reader) ([]byte, error) {
	zw, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	content, err := io.ReadAll(zw)
	if err != nil {
		return nil, fmt.Errorf("failed to uncompress: %w", err)
	}
	return content, nil
}
