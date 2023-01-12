package v1alpha1

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MutationSpec defines a common spec between Localization and Configuration.
type MutationSpec struct {
	// +required
	Interval metav1.Duration `json:"interval"`

	// +required
	Source Source `json:"source"`

	// +required
	ConfigRef ConfigReference `json:"configRef"`

	// +required
	SnapshotTemplate SnapshotTemplateSpec `json:"snapshotTemplate"`

	// +optional
	Values map[string]string `json:"values,omitempty"`
}

// MutationStatus defines a common status for Localizations and Configurations.
type MutationStatus struct {
	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LatestSnapshotDigest string `json:"latestSnapshotDigest,omitempty"`

	LatestConfigVersion string `json:"latestConfigVersion,omitempty"`
}

type ConfigReference struct {
	// +required
	ComponentVersionRef meta.NamespacedObjectReference `json:"componentVersionRef"`

	// +required
	Resource Source `json:"resource"`
}

// ResourceRef define a resource.
// TODO: Change this to ocmmetav1.ResourceReference
// The ocmmetav1.ResourceReference can also contain version!
type ResourceRef struct {
	// +required
	Name string `json:"name"`
	// +optional
	Version string `json:"version,omitempty"`

	// +optional
	ExtraIdentity map[string]string `json:"extraIdentity,omitempty"`

	// ReferencePath is a list of references with identities that include this resource.
	//      referencePath:
	//        - name: installer
	// +optional
	ReferencePath []map[string]string `json:"referencePath,omitempty"`
}

// Source defines a possible incoming format for sources that this object needs for further configuration/localization
// steps.
// +kubebuilder:validation:MinProperties=1
type Source struct {
	// +optional
	SourceRef *meta.NamespacedObjectKindReference `json:"sourceRef,omitempty"`
	// +optional
	ResourceRef *ResourceRef `json:"resourceRef,omitempty"`
}

// GetSourceSnapshotKey is a convenient wrapper to get the NamespacedName for a snapshot reference on the object.
func (in MutationSpec) GetSourceSnapshotKey() types.NamespacedName {
	if in.Source.SourceRef == nil {
		return types.NamespacedName{}
	}
	return types.NamespacedName{
		Namespace: in.Source.SourceRef.Namespace,
		Name:      in.Source.SourceRef.Name,
	}
}

// GetRequeueAfter returns the duration after which the Localization must be
// reconciled again.
func (in MutationSpec) GetRequeueAfter() time.Duration {
	return in.Interval.Duration
}
