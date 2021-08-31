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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// Inventory is a record of objects that are applied on a cluster.
type Inventory struct {
	Entries map[string]string `json:"entries"`
}

func NewInventory() *Inventory {
	return &Inventory{Entries: map[string]string{}}
}

// Add adds the given objects to the inventory.
func (inv *Inventory) Add(objects []*unstructured.Unstructured) error {
	for _, om := range objects {
		objMeta := object.UnstructuredToObjMeta(om)
		gv, err := schema.ParseGroupVersion(om.GetAPIVersion())
		if err != nil {
			return err
		}
		inv.Entries[objMeta.String()] = gv.Version
	}

	return nil
}

// List returns the inventory entries as unstructured.Unstructured objects.
func (inv *Inventory) List() ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	list, err := inv.ListMeta()
	if err != nil {
		return nil, err
	}

	for _, metadata := range list {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   metadata.GroupKind.Group,
			Kind:    metadata.GroupKind.Kind,
			Version: inv.Entries[metadata.String()],
		})
		u.SetName(metadata.Name)
		u.SetNamespace(metadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(InventoryOrder(objects))
	return objects, nil
}

// ListMeta returns the inventory entries as object.ObjMetadata objects.
func (inv *Inventory) ListMeta() ([]object.ObjMetadata, error) {
	var metas []object.ObjMetadata
	for e, _ := range inv.Entries {
		m, err := object.ParseObjMetadata(e)
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
			Version: inv.Entries[metadata.String()],
		})
		u.SetName(metadata.Name)
		u.SetNamespace(metadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(InventoryOrder(objects))
	return objects, nil
}
