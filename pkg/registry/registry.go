// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
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

		if req.Method != "GET" || req.URL.Path == "/v2/" {
			h.ServeHTTP(w, req)
			return
		}

		if strings.Contains(req.URL.Path, "/blobs/") {
			if req.Header.Get("X-Repository") == "" || req.Header.Get("X-Registry") == "" || req.Header.Get("X-Tag") == "" {
				log.Error("headers missing")
				h.ServeHTTP(w, req)
				return
			}

			repo := strings.Replace(req.Header.Get("X-Repository"), req.Header.Get("X-Registry"), addr, 1)
			image := fmt.Sprintf("%s:%s", repo, req.Header.Get("X-Tag"))
			ref, err := name.ParseReference(image)
			if err != nil {
				log.Errorf("could not parse reference: %s", err)
				h.ServeHTTP(w, req)
				return
			}

			if _, err := remote.Get(ref, remote.WithAuthFromKeychain(keychain)); err != nil {
				switch err.(type) {
				case *transport.Error:
					if err.(*transport.Error).StatusCode == http.StatusNotFound {
						log.Info("caching image ", "image", image)
						src := fmt.Sprintf("%s:%s", req.Header.Get("X-Repository"), req.Header.Get("X-Tag"))
						if err := crane.Copy(src, image, crane.WithAuthFromKeychain(keychain)); err != nil {
							log.Errorf("could not copy image: %w", err)
						}
					}
				}
			}
		}

		h.ServeHTTP(w, req)
	})
}
