package untar

import (
	"compress/gzip"
	"fmt"
	"io"
)

type PlainGzipDecompress struct{}

func (*PlainGzipDecompress) Untar(in io.Reader) ([]byte, error) {
	zw, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("cannot create gzip: %w", err)
	}

	content, err := io.ReadAll(zw)
	if err != nil {
		return nil, fmt.Errorf("failed to uncompress: %w", err)
	}
	return content, nil
}
