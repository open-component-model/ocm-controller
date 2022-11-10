package controllers

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

func TestGetNestedComponentDescriptor(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	assert.NoError(t, err)
	fakeClient := fake.NewClientBuilder()

	var (
		componentName = "test-name"
		namespace     = "default"
		notNestedName = "not-nested-component"
	)
	obj := &v1alpha1.ComponentVersion{
		Status: v1alpha1.ComponentVersionStatus{
			ComponentDescriptor: v1alpha1.Reference{
				Name: "not-reference",
				References: []v1alpha1.Reference{
					{
						Name: "nested-once",
						References: []v1alpha1.Reference{
							{
								Name: "nested-twice",
								ComponentDescriptorRef: meta.NamespacedObjectReference{
									Name:      "not-component",
									Namespace: namespace,
								},
							},
							{
								Name: "nested-twice-second",
								ComponentDescriptorRef: meta.NamespacedObjectReference{
									Name:      componentName,
									Namespace: namespace,
								},
							},
						},
					},
				},
				ExtraIdentity: nil,
				ComponentDescriptorRef: meta.NamespacedObjectReference{
					Name:      notNestedName,
					Namespace: namespace,
				},
			},
		},
	}
	componentDesc := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
	}
	notNestedComponentDesc := &v1alpha1.ComponentDescriptor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      notNestedName,
			Namespace: namespace,
		},
	}
	client := fakeClient.WithObjects(componentDesc, notNestedComponentDesc).WithScheme(scheme).Build()
	locr := &LocalizationReconciler{
		Scheme: scheme,
		Client: client,
	}

	t.Run("with reference path", func(t *testing.T) {
		loc := &v1alpha1.Localization{
			Spec: v1alpha1.LocalizationSpec{
				ConfigRef: v1alpha1.ConfigReference{
					ReferencePath: "nested-twice-second",
				},
			},
		}
		comp, err := locr.getComponentDescriptor(context.Background(), loc, obj.Status.ComponentDescriptor)
		assert.NoError(t, err)
		assert.Equal(t, componentName, comp.Name)
	})

	t.Run("without reference path", func(t *testing.T) {
		loc := &v1alpha1.Localization{
			Spec: v1alpha1.LocalizationSpec{
				ConfigRef: v1alpha1.ConfigReference{},
			},
		}
		comp, err := locr.getComponentDescriptor(context.Background(), loc, obj.Status.ComponentDescriptor)
		assert.NoError(t, err)
		assert.Equal(t, notNestedName, comp.Name)
	})
}
