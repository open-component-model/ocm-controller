package controllers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

// Untar takes a reader and extracts the content into memory.
// This function should only be used in tests for extracting simple content.
func Untar(in io.ReadCloser) ([]byte, error) {
	var result []byte
	buffer := bytes.NewBuffer(result)
	zr, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("requires gzip-compressed body: %v", err)
	}
	tr := tar.NewReader(zr)

	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
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
