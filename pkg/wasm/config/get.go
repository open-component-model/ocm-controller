//go:build tinygo.wasm

package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Name returns the name of the OCM resource being transformed
func Name() string {
	if len(os.Args) == 0 {
		return ""
	}
	return os.Args[0]
}

// Get returns the configuration data passed to WASM Module
func Get() map[string]string {
	if len(os.Args) == 0 {
		return nil
	}
	payload := []byte(os.Args[1])

	var config map[string]string
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return nil
	}

	return config
}
