// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// GetSnapshotNameForObject returns an existing snapshot name or generates a random one in the case
// that the name does not exist
func GetSnapshotNameForObject(name string, obj v1alpha1.SnapshotProducer) (string, error) {
	if obj.GetSnapshotName() != "" {
		return obj.GetSnapshotName(), nil
	}
	b := make([]byte, 5)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	randomString := strings.ToLower(base32.StdEncoding.EncodeToString(b)[:7])
	return fmt.Sprintf("%s-%s", name, randomString), nil
}
