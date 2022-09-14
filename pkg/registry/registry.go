// Modified from https://github.com/mesosphere/mindthegap/blob/main/docker/registry/registry.go
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/sirupsen/logrus"
)

type Config struct {
	StorageDirectory string
	Addr             string
}

func (c Config) ToRegistryConfiguration() (*configuration.Configuration, error) {
	registryConfigString, err := registryConfiguration(c)
	if err != nil {
		return nil, err
	}

	registryConfig, err := configuration.Parse(strings.NewReader(registryConfigString))
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry configuration: %w", err)
	}
	return registryConfig, nil
}

func registryConfiguration(c Config) (string, error) {
	configTmpl := `
version: 0.1
storage:
  filesystem:
    rootdirectory: {{ .StorageDirectory }}
  maintenance:
    uploadpurging:
      enabled: false
    readonly:
      enabled: false
http:
  net: tcp
  addr: {{ .Addr }}
log:
  accesslog:
    disabled: true
  level: error
`
	tmpl := template.New("registryConfig")
	template.Must(tmpl.Parse(configTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		StorageDirectory string
		Addr             string
	}{c.StorageDirectory, c.Addr}); err != nil {
		return "", fmt.Errorf("failed to render registry configuration: %w", err)
	}

	return buf.String(), nil
}

type Registry struct {
	config   *configuration.Configuration
	delegate *http.Server
	address  string
}

func NewRegistry(cfg Config) (*Registry, error) {
	registryConfig, err := cfg.ToRegistryConfiguration()
	if err != nil {
		return nil, err
	}

	logrus.SetLevel(logrus.FatalLevel)
	regHandler := handlers.NewApp(context.Background(), registryConfig)

	reg := &http.Server{
		Addr:              registryConfig.HTTP.Addr,
		Handler:           regHandler,
		ReadHeaderTimeout: 1 * time.Second,
	}

	return &Registry{
		config:   registryConfig,
		delegate: reg,
		address:  registryConfig.HTTP.Addr,
	}, nil
}

func (r Registry) Address() string {
	return r.address
}

func (r Registry) Shutdown(ctx context.Context) error {
	return r.delegate.Shutdown(ctx)
}

func (r Registry) ListenAndServe() error {
	var err error
	err = r.delegate.ListenAndServe()

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
