package untar

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-logr/logr"
)

// Untarer will take a reader and will try a number of configured readers in order
// as configured to untar it. If everything fails, it will throw an error.
// The reader is created and the untar uses TeeReader to not close the original
// reader so the next method can read it again.
type Untarer interface {
	Untar(in io.Reader) ([]byte, error)
}

type Method struct {
	Name   string
	Method Untarer
}

// FallbackUntar will attempt to decompress an archive, or give back the data as is.
type FallbackUntar struct {
	methods []Method
	logger  logr.Logger
}

// NewFallbackUntar creates a new fallback untarer.
func NewFallbackUntar(log logr.Logger, methods ...Method) *FallbackUntar {
	return &FallbackUntar{
		logger:  log.WithName("untarer"),
		methods: methods,
	}
}

// Untar expects a small content so for brevity it will copy all information from the reader into memory
// instead of using a reusable reader with TeeReader.
func (f *FallbackUntar) Untar(in io.Reader) ([]byte, error) {
	readerContent, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from reader: %w", err)
	}
	for _, method := range f.methods {
		f.logger.Info("trying untar method", "method", method.Name)
		content, err := method.Method.Untar(bytes.NewBuffer(readerContent))
		if err != nil {
			//f.logger.Error(err, "method failed, trying next: ")
			continue
		}
		return content, nil
	}

	return nil, fmt.Errorf("none of the configured methods were able to do something with the content")
}
