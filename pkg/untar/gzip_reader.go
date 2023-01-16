package untar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

// GzipFallbackUntarer tries to unpack an archive using tar gzip.
type GzipFallbackUntarer struct{}

func (g *GzipFallbackUntarer) Untar(in io.ReadCloser) ([]byte, error) {
	var result []byte
	buffer := bytes.NewBuffer(result)
	zr, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("requires gzip-compressed body: %v", err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("invalid header in tar: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			if _, err := io.Copy(buffer, tr); err != nil {
				return nil, fmt.Errorf("unable to copy tar file to filesystem: %w", err)
			}
		}
	}

	return buffer.Bytes(), nil
}
