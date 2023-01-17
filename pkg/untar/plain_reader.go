package untar

import "io"

type PlainReader struct{}

func (p *PlainReader) Untar(in io.Reader) ([]byte, error) {
	return io.ReadAll(in)
}
