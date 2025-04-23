package component

import (
	"fmt"
	"strings"

	hash "github.com/mitchellh/hashstructure"
	v1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
)

type namingScheme struct {
	ComponentName string
	Version       string
	Identity      v1.Identity
}

// ConstructUniqueName for a given component descriptor based on metadata that can be used to uniquely identify components.
func ConstructUniqueName(name, version string, identity v1.Identity) (string, error) {
	h, err := hash.Hash(namingScheme{
		ComponentName: name,
		Version:       version,
		Identity:      identity,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate hash for name, version, identity: %w", err)
	}

	return fmt.Sprintf("%s-%s-%d", strings.ReplaceAll(name, "/", "-"), version, h), nil
}
