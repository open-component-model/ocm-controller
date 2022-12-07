// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
)

// Verifier is a wrapper around go-digest.Verifier
type Verifier struct {
	// Digest is the digest of the blob to verify.
	digest   string
	verifier digest.Verifier
}

// NewVerifier returns a new Verifier. It accepts a string, which is the digest of the blob to verify.
// The digest must be of the form <algorithm>:<hex>.
// The returned Verifier can be used to verify the blob.
func NewVerifier(d string) *Verifier {
	dig := digest.Digest(d)

	return &Verifier{
		digest:   d,
		verifier: dig.Verifier(),
	}
}

// Verify verifies the blob. It accepts an io.Reader, which is the blob to verify.
// It returns a boolean, which is true if the blob is verified, and an error.
func (v *Verifier) Verify(rd io.ReadCloser) (bool, error) {
	if _, err := io.Copy(v.verifier, rd); err != nil {
		return false, fmt.Errorf("failed to verify blob: %w", err)
	}
	return v.verifier.Verified(), nil
}
