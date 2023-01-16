package untar

import "io"

type PlainReader struct{}

func (p *PlainReader) Untar(in io.ReadCloser) ([]byte, error) {
	return io.ReadAll(in)
}
