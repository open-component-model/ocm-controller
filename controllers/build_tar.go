package controllers

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BuildTar is a modified version of https://github.com/fluxcd/pkg/blob/2ee90dd5b2ec033f44881f160e29584cceda8f37/oci/client/build.go
func BuildTar(artifactPath, sourceDir string) error {
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("invalid source dir path: %s", sourceDir)
	}

	tf, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	tmpName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	gw := gzip.NewWriter(tf)
	tw := tar.NewWriter(gw)

	if err := filepath.Walk(sourceDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore anything that is not a file or directories e.g. symlinks
		if m := fi.Mode(); !(m.IsRegular() || m.IsDir()) {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}
		// The name needs to be modified to maintain directory structure
		// as tar.FileInfoHeader only has access to the base name of the file.
		// Ref: https://golang.org/src/archive/tar/common.go?#L626
		relFilePath := p
		if filepath.IsAbs(sourceDir) {
			relFilePath, err = filepath.Rel(sourceDir, p)
			if err != nil {
				return err
			}
		}
		header.Name = relFilePath

		// Remove any environment specific data.
		header.Gid = 0
		header.Uid = 0
		header.Uname = ""
		header.Gname = ""
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}); err != nil {
		tw.Close()
		gw.Close()
		tf.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		gw.Close()
		tf.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		gw.Close()
		tf.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, 0o640); err != nil {
		return err
	}

	return os.Rename(tmpName, artifactPath)
}
