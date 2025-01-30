package controllers

import (
	"archive/tar"
	"bytes"
)

// isTar checks if a given content is a tar archive or not.
func isTar(content []byte) bool {
	tr := tar.NewReader(bytes.NewBuffer(content))
	_, err := tr.Next()

	return err == nil
}
