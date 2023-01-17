package untar

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// PlainUntarer will attempt a plain untar operation and not gzipped.
type PlainUntarer struct{}

func (*PlainUntarer) Untar(in io.Reader) ([]byte, error) {
	var result []byte
	buffer := bytes.NewBuffer(result)
	tr := tar.NewReader(in)

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
