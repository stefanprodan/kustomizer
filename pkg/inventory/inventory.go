/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package inventory

import (
	"sort"

	"github.com/stefanprodan/kustomizer/pkg/objectutil"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// Inventory is a record of objects that are applied on a cluster stored as a configmap.
type Inventory struct {
	// Name of the inventory configmap.
	Name string

	// Namespace of the inventory configmap.
	Namespace string

	// Entries of Kubernetes objects metadata.
	Entries []Entry `json:"entries"`
}

// Entry contains the information necessary to locate the
// resource within a cluster.
type Entry struct {
	// ObjectID is the string representation of object.ObjMetadata,
	// in the format '<namespace>_<name>_<group>_<kind>'.
	ObjectID string `json:"id"`

	// ObjectVersion is the API version of this entry kind.
	ObjectVersion string `json:"ver"`
}

func NewInventory(name, namespace string) *Inventory {
	return &Inventory{
		Name:      name,
		Namespace: namespace,
		Entries:   []Entry{},
	}
}

// AddObjects extracts the metadata from the given objects and adds it to the inventory.
func (inv *Inventory) AddObjects(objects []*unstructured.Unstructured) error {
	sort.Sort(objectutil.SortableUnstructureds(objects))
	for _, om := range objects {
		objMetadata := object.UnstructuredToObjMeta(om)
		gv, err := schema.ParseGroupVersion(om.GetAPIVersion())
		if err != nil {
			return err
		}

		inv.Entries = append(inv.Entries, Entry{
			ObjectID:      objMetadata.String(),
			ObjectVersion: gv.Version,
		})
	}

	return nil
}

// VersionOf returns the API version of the given object if found in this inventory.
func (inv *Inventory) VersionOf(objMetadata object.ObjMetadata) string {
	for _, entry := range inv.Entries {
		if entry.ObjectID == objMetadata.String() {
			return entry.ObjectVersion
		}
	}
	return ""
}

// ListObjects returns the inventory entries as unstructured.Unstructured objects.
func (inv *Inventory) ListObjects() ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)

	for _, entry := range inv.Entries {
		objMetadata, err := object.ParseObjMetadata(entry.ObjectID)
		if err != nil {
			return nil, err
		}

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   objMetadata.GroupKind.Group,
			Kind:    objMetadata.GroupKind.Kind,
			Version: entry.ObjectVersion,
		})
		u.SetName(objMetadata.Name)
		u.SetNamespace(objMetadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(objectutil.SortableUnstructureds(objects))
	return objects, nil
}

// ListMeta returns the inventory entries as object.ObjMetadata objects.
func (inv *Inventory) ListMeta() ([]object.ObjMetadata, error) {
	var metas []object.ObjMetadata
	for _, e := range inv.Entries {
		m, err := object.ParseObjMetadata(e.ObjectID)
		if err != nil {
			return metas, err
		}
		metas = append(metas, m)
	}

	return metas, nil
}

// Diff returns the slice of objects that do not exist in the target inventory.
func (inv *Inventory) Diff(target *Inventory) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	aList, err := inv.ListMeta()
	if err != nil {
		return nil, err
	}

	bList, err := target.ListMeta()
	if err != nil {
		return nil, err
	}

	list := object.SetDiff(aList, bList)
	if len(list) == 0 {
		return objects, nil
	}

	for _, metadata := range list {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   metadata.GroupKind.Group,
			Kind:    metadata.GroupKind.Kind,
			Version: inv.VersionOf(metadata),
		})
		u.SetName(metadata.Name)
		u.SetNamespace(metadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(objectutil.SortableUnstructureds(objects))
	return objects, nil
}
