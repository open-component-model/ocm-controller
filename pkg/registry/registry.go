// Modified from https://github.com/mesosphere/mindthegap/blob/main/docker/registry/registry.go
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// New creates a new docker registry server
func New(ctx context.Context, addr string) *http.Server {
	return newRegistry(ctx, addr)
}

func newRegistry(ctx context.Context, addr string) *http.Server {
	config := getConfig(fmt.Sprintf(":%s", addr))
	app := handlers.NewApp(ctx, config)
	logger := dcontext.GetLogger(app)
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", addr),
		Handler:           pullThroughMiddleware(app, fmt.Sprintf("127.0.0.1:%s", addr), logger),
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

func pullThroughMiddleware(h http.Handler, addr string, log dcontext.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL.Host = addr
		req.Host = addr

		if req.Method != "GET" {
			h.ServeHTTP(w, req)
			return
		}

		if req.URL.Path == "/v2/" {
			h.ServeHTTP(w, req)
			return
		}

		if strings.Contains(req.URL.Path, "/blobs/") {
			if req.Header.Get("X-Repository") == "" || req.Header.Get("X-Registry") == "" || req.Header.Get("X-Tag") == "" {
				log.Error("headers missing")
				h.ServeHTTP(w, req)
				return
			}

			log.Info("GOT REPOSITORY", req.Header.Get("X-Repository"))
			log.Info("GOT REGISTRY", req.Header.Get("X-Registry"))
			log.Info("GOT TAG", req.Header.Get("X-Tag"))
			log.Info("HAVE ADDR", addr)

			repo := strings.Replace(req.Header.Get("X-Repository"), req.Header.Get("X-Registry"), addr, 1)

			log.Info("MADE REPO", repo)

			image := fmt.Sprintf("%s:%s", repo, req.Header.Get("X-Tag"))
			ref, err := name.ParseReference(image)
			if err != nil {
				log.Errorf("could not parse reference: %s", err)
				h.ServeHTTP(w, req)
				return
			}

			log.Info("checking image ", image)

			if _, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
				switch err.(type) {
				case *transport.Error:
					if err.(*transport.Error).StatusCode == http.StatusNotFound {
						log.Info("caching image", "image", image)
						src := fmt.Sprintf("%s:%s", req.Header.Get("X-Repository"), req.Header.Get("X-Tag"))
						if err := crane.Copy(src, image); err != nil {
							log.Errorf("could not copy image: %w", err)
						}
					}
				}
			}
		}

		h.ServeHTTP(w, req)
	})
}
