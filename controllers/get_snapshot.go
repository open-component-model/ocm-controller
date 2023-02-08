package controllers

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

// CheckIfSnapshotExists returns if the snapshot exists or not.
// We don't care about the state of the snapshot to not potentially, overwrite an in-progress reconciliation.
func CheckIfSnapshotExists(ctx context.Context, client client.Client, name, namespace string) (*v1alpha1.Snapshot, error) {
	snapshot := &v1alpha1.Snapshot{}
	if err := client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get snapshot for localization object: %w", err)
	}

	return snapshot, nil
}
