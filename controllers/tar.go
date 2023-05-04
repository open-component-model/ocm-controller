// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

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
