// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
)

// New creates a new docker registry server
func New(ctx context.Context, addr, namespace, serviceAccount string) *http.Server {
	return newRegistry(ctx, addr, namespace, serviceAccount)
}

func newRegistry(ctx context.Context, addr, namespace, serviceAccount string) *http.Server {
	config := getConfig(fmt.Sprintf(":%s", addr))
	app := handlers.NewApp(ctx, config)
	logger := dcontext.GetLogger(app)
	keychain, err := k8schain.NewInCluster(context.Background(), k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
	})
	if err != nil {
		log.Fatalf("k8schain.New() = %v", err)
	}
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", addr),
		Handler:           pullThroughMiddleware(app, fmt.Sprintf("127.0.0.1:%s", addr), logger, keychain),
		ReadHeaderTimeout: 1 * time.Second,
	}
}

func getConfig(addr string) *configuration.Configuration {
	config := &configuration.Configuration{}
	config.HTTP.Addr = addr
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{
		"filesystem": map[string]interface{}{
			"rootdirectory": "/tmp",
		},
	}
	return config
}

func pullThroughMiddleware(h http.Handler, addr string, log dcontext.Logger, keychain authn.Keychain) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL.Host = addr
		req.Host = addr
		h.ServeHTTP(w, req)
	})
}
