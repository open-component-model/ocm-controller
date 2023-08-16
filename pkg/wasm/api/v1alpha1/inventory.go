/*
Copyright 2023 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const fmtSeparator = "/"

// ResourceInventory contains a list of Kubernetes resource object references
// that have been applied by a Kustomization.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type ResourceInventory struct {
	// Entries of Kubernetes resource object references.
	Entries []ResourceRef `json:"entries"`
}

// ResourceRef contains the information necessary to locate a resource within a cluster.
type ResourceRef struct {
	ID string `json:"id"`

	Version string `json:"v"`
}

// AddChangeSet extracts the metadata from the given objects and adds it to the inventory.
func (ri *ResourceInventory) AddChangeSet(set *ChangeSet) error {
	if set == nil {
		return nil
	}

	for _, entry := range set.Entries() {
		ri.Entries = append(ri.Entries, ResourceRef{
			ID:      entry.ObjMetadata.String(),
			Version: entry.GroupVersion,
		})
	}

	return nil
}

// ChangeSetEntry defines the result of an action performed on an object.
type ChangeSetEntry struct {
	// ObjMetadata holds the unique identifier of this entry.
	ObjMetadata object.ObjMetadata

	// GroupVersion holds the API group version of this entry.
	GroupVersion string

	// Subject represents the Object ID in the format 'kind/namespace/name'.
	Subject string
}

// FmtObjMetadata returns the object ID in the format <kind>/<namespace>/<name>.
func FmtObjMetadata(obj object.ObjMetadata) string {
	var builder strings.Builder
	builder.WriteString(obj.GroupKind.Kind + fmtSeparator)
	if obj.Namespace != "" {
		builder.WriteString(obj.Namespace + fmtSeparator)
	}
	builder.WriteString(obj.Name)
	return builder.String()
}

// FmtUnstructured returns the object ID in the format <kind>/<namespace>/<name>.
func FmtUnstructured(obj *unstructured.Unstructured) string {
	return FmtObjMetadata(object.UnstructuredToObjMetadata(obj))
}

type ChangeSet struct {
	entries []*ChangeSetEntry
}

func NewChangeSet() *ChangeSet {
	return &ChangeSet{
		entries: make([]*ChangeSetEntry, 0),
	}
}

func (c *ChangeSet) AddEntry(e *unstructured.Unstructured) {
	c.entries = append(c.entries, changeSetEntry(e))
}

func (c *ChangeSet) Entries() []*ChangeSetEntry {
	return c.entries
}

func changeSetEntry(o *unstructured.Unstructured) *ChangeSetEntry {
	return &ChangeSetEntry{
		ObjMetadata:  object.UnstructuredToObjMetadata(o),
		GroupVersion: o.GroupVersionKind().Version,
		Subject:      FmtUnstructured(o),
	}
}

func (e ChangeSetEntry) String() string {
	return e.Subject
}
