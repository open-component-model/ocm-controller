package v1alpha1

// SnapshotTemplateSpec defines the template used to create snapshots.
type SnapshotTemplateSpec struct {
	// +required
	Name string `json:"name"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
