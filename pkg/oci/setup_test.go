// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/phayes/freeport"
)

var testServer *httptest.Server

// Server is a registry server
// It wraps the http.Server
type Server struct {
	http.Server
	logger dcontext.Logger
	config *configuration.Configuration
}

// New creates a new oci registry server
func New(ctx context.Context, addr string) (*Server, error) {
	config, err := getConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("could not get config: %w", err)
	}
	app := handlers.NewApp(ctx, config)
	logger := dcontext.GetLogger(app)
	return &Server{
		http.Server{
			Addr:              addr,
			Handler:           app,
			ReadHeaderTimeout: 1 * time.Second,
		},
		logger,
		config,
	}, nil
}

func getConfig(addr string) (*configuration.Configuration, error) {
	config := &configuration.Configuration{}
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	return config, nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	port, err := freeport.GetFreePort()
	if err != nil {
		panic(fmt.Errorf("could not get free port: %w", err))
	}
	app, err := New(ctx, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		panic(fmt.Errorf("could not create registry server: %w", err))
	}
	testServer = httptest.NewServer(app.Handler)
	defer testServer.Close()
	exitCode := m.Run()
	os.Exit(exitCode)
}

func generateRandomName(name string) string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s-%d", name, rand.Intn(1000))
}
