package registry

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"
)

type OCIClient struct {
	url    string
	client *client.Client
}

func NewClient(url string) *OCIClient {
	options := []crane.Option{
		crane.WithUserAgent("ocm-controller/v1"),
	}
	client := client.NewClient(options)
	return &OCIClient{
		url:    url,
		client: client,
	}
}

// Push takes a path, creates an archive of the files in it and pushes the content to the OCI registry.
func (o *OCIClient) Push(ctx context.Context, artifactPath, sourcePath string, metadata client.Metadata) error {
	if err := o.client.Build(artifactPath, sourcePath, nil); err != nil {
		return fmt.Errorf("failed to create archive of the fetched artifacts: %w", err)
	}
	if _, err := o.client.Push(ctx, o.url, sourcePath, metadata, nil); err != nil {
		return fmt.Errorf("failed to push oci image: %w", err)
	}
	return nil
}
