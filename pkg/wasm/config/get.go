package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Name returns the name of the OCM resource being transformed.
func Name() string {
	if len(os.Args) == 0 {
		return ""
	}

	return os.Args[0]
}

// DataDir returns the path to a mounted directory or an
// empty string if not directory is mounted.
func DataDir() string {
	return os.Getenv("OCM_SOFTWARE_DATA_DIR")
}

// Get returns the configuration data passed to WASM Module.
func Get() (map[string]string, error) {
	if len(os.Args) == 0 {
		return nil, errors.New("config not available")
	}

	payload := []byte(os.Args[1])

	var config map[string]string
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// GetValue returns the configuration data passed to WASM Module.
func GetValue(key string) (string, error) {
	if len(os.Args) <= 1 {
		return "", errors.New("config not available")
	}
	payload := []byte(os.Args[1])

	var config map[string]string
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return "", fmt.Errorf("failed to decode config: %w", err)
	}

	val, ok := config[key]
	if !ok {
		return "", fmt.Errorf("value %s not found", key)
	}

	return val, nil
}
