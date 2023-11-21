// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

// GenerateSnapshotName generates a random snapshot name.
func GenerateSnapshotName(name string) (string, error) {
	const size = 5

	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	const offset = 7
	randomString := strings.ToLower(base32.StdEncoding.EncodeToString(b)[:offset])

	return fmt.Sprintf("%s-%s", name, randomString), nil
}
