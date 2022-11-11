// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get uses the client and reference to get an external, unstructured object.
func Get(ctx context.Context, c client.Reader, ref *corev1.ObjectReference, namespace string) (*unstructured.Unstructured, error) {
	if ref == nil {
		return nil, fmt.Errorf("cannot get object - object reference not set")
	}
	obj := new(unstructured.Unstructured)
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetName(ref.Name)
	key := client.ObjectKey{Name: obj.GetName(), Namespace: namespace}
	if err := c.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to retrieve %s external object %q/%q: %w", obj.GetKind(), key.Namespace, key.Name, err)
	}
	return obj, nil
}
