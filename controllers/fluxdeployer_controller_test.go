package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"github.com/open-component-model/ocm-controller/pkg/cache/fakes"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
)

func TestFluxDeployerReconcile(t *testing.T) {
	resourceV1 := &v1alpha1.Resource{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Status: v1alpha1.ResourceStatus{
			SnapshotName: "test-snapshot",
		},
	}
	deployer := &v1alpha1.FluxDeployer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployer",
			Namespace: "default",
		},
		Spec: v1alpha1.FluxDeployerSpec{
			SourceRef: v1alpha1.ObjectReference{
				NamespacedObjectKindReference: meta.NamespacedObjectKindReference{
					Name:      "test-resource",
					Namespace: "default",
					Kind:      "Resource",
				},
			},
			HelmReleaseTemplate: &helmv2.HelmReleaseSpec{
				Chart: &helmv2.HelmChartTemplate{
					Spec: helmv2.HelmChartTemplateSpec{
						Chart:   "podinfo",
						Version: "6.3.5",
					},
				},
			},
		},
	}
	conditions.MarkTrue(deployer, meta.ReadyCondition, meta.SucceededReason, "done")
	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "default",
		},
		Spec: v1alpha1.SnapshotSpec{
			Identity: ocmmetav1.Identity{
				v1alpha1.ComponentNameKey:         "component-name",
				v1alpha1.ComponentVersionKey:      "v0.0.1",
				v1alpha1.ResourceNameKey:          "resource-name",
				v1alpha1.ResourceVersionKey:       "v0.0.5",
				v1alpha1.ResourceHelmChartVersion: "v0.0.5",
			},
			Digest: "digest-1",
			Tag:    "1234",
		},
		Status: v1alpha1.SnapshotStatus{
			LastReconciledDigest: "digest-1",
			LastReconciledTag:    "1234",
		},
	}
	conditions.MarkTrue(snapshot, meta.ReadyCondition, meta.SucceededReason, "Snapshot with name '%s' is ready", snapshot.Name)

	client := env.FakeKubeClient(
		WithAddToScheme(helmv2.AddToScheme),
		WithAddToScheme(sourcev1beta2.AddToScheme),
		WithAddToScheme(kustomizev1.AddToScheme),
		WithObjects(snapshot, deployer, resourceV1),
	)
	fakeCache := &fakes.FakeCache{}
	content, err := os.Open(filepath.Join("testdata", "podinfo-6.3.5.tgz"))
	require.NoError(t, err)
	fakeCache.FetchDataByDigestReturns(content, nil)
	recorder := record.NewFakeRecorder(32)
	dc := env.FakeDynamicKubeClient(WithObjects(snapshot, deployer, resourceV1))

	sr := FluxDeployerReconciler{
		Client:              client,
		Scheme:              env.scheme,
		EventRecorder:       recorder,
		RegistryServiceName: "127.0.0.1:5000",
		RetryInterval:       0,
		DynamicClient:       dc,
		Cache:               fakeCache,
	}

	result, err := sr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      deployer.Name,
			Namespace: deployer.Namespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	close(recorder.Events)
}
