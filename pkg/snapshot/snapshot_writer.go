package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache"
	"github.com/open-component-model/ocm-controller/pkg/ocm"
)

// Writer creates a snapshot using an artifact path as location for the snapshot
// data.
type Writer interface {
	Write(ctx context.Context, owner v1alpha1.SnapshotWriter, sourceDir string, identity ocmmetav1.Identity) (string, int64, error)
}

// OCIWriter writes snapshot data into the cluster-local OCI cache.
type OCIWriter struct {
	Client client.Client
	Cache  cache.Cache
	Scheme *runtime.Scheme
}

// NewOCIWriter creates a new OCI cache writer.
func NewOCIWriter(client client.Client, cache cache.Cache, scheme *runtime.Scheme) *OCIWriter {
	return &OCIWriter{
		Client: client,
		Cache:  cache,
		Scheme: scheme,
	}
}

func (w *OCIWriter) Write(
	ctx context.Context,
	owner v1alpha1.SnapshotWriter,
	sourceDir string,
	identity ocmmetav1.Identity,
) (_ string, _ int64, err error) {
	logger := log.FromContext(ctx).WithName("snapshot-writer")

	logger.V(v1alpha1.LevelDebug).Info("creating snapshot for identity", "identity", identity)
	artifactPath, err := os.CreateTemp("", "snapshot-artifact-*.tgz")
	if err != nil {
		return "", -1, fmt.Errorf("fs error: %w", err)
	}

	if err := buildTar(artifactPath.Name(), sourceDir); err != nil {
		return "", -1, fmt.Errorf("build tar error: %w", err)
	}

	logger.V(v1alpha1.LevelDebug).Info("built tar file")

	file, err := os.Open(artifactPath.Name())
	if err != nil {
		return "", -1, fmt.Errorf("failed to open created archive: %w", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			err = errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(artifactPath.Name()); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
	}()

	name, err := ocm.ConstructRepositoryName(identity)
	if err != nil {
		return "", -1, fmt.Errorf("failed to construct name: %w", err)
	}

	logger.V(v1alpha1.LevelDebug).Info("repository name constructed", "name", name)

	tag := owner.GetResourceVersion()
	if v, ok := identity[v1alpha1.ResourceHelmChartVersion]; ok {
		tag = v
	} else if v, ok = identity[v1alpha1.ResourceVersionKey]; ok {
		// 2024-07-10 d :
		// Partial fix for #68.   If available, default to using the resource
		// version as the tag.
		tag = v
	}

	snapshotDigest, size, err := w.Cache.PushData(ctx, file, "", name, tag)
	if err != nil {
		return "", -1, fmt.Errorf("failed to push blob to local registry: %w", err)
	}

	logger.V(v1alpha1.LevelDebug).Info("pushed data to the cache with digest", "digest", snapshotDigest)

	snapshotCR := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.GetSnapshotName(),
			Namespace: owner.GetNamespace(),
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, w.Client, snapshotCR, func() error {
		if snapshotCR.ObjectMeta.CreationTimestamp.IsZero() {
			if err := controllerutil.SetOwnerReference(owner, snapshotCR, w.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference on snapshot: %w", err)
			}
		}
		snapshotCR.Spec = v1alpha1.SnapshotSpec{
			Identity: identity,
			Digest:   snapshotDigest,
			Tag:      tag,
		}

		return nil
	})
	if err != nil {
		return "", -1, fmt.Errorf("failed to create or update component descriptor: %w", err)
	}

	logger.Info("snapshot successfully created/updated", "digest", snapshotDigest, "snapshot", snapshotCR)

	return snapshotDigest, size, nil
}
