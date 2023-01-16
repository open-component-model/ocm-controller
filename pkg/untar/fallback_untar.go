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
	Untar(in io.ReadCloser) ([]byte, error)
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

func (f *FallbackUntar) Untar(in io.ReadCloser) ([]byte, error) {
	defer in.Close()
	for _, method := range f.methods {
		f.logger.Info("trying untar method", "method", method.Name)
		buf := &bytes.Buffer{}
		tee := io.TeeReader(in, buf)
		content, err := method.Method.Untar(io.NopCloser(tee))
		if err != nil {
			f.logger.Error(err, "method failed, trying next: ")
			continue
		}
		return content, nil
	}

	return nil, fmt.Errorf("none of the configured methods were able to do something with the content")
}
