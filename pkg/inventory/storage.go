/*
Copyright 2021 Stefan Prodan

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
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/ssa"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	InventoryKindName = "inventory"
	InventoryPrefix   = "inv-"
	nameLabelKey      = "app.kubernetes.io/name"
	componentLabelKey = "app.kubernetes.io/component"
	createdByLabelKey = "app.kubernetes.io/created-by"
)

// Storage manages the Inventory in-cluster storage.
type Storage struct {
	Manager *ssa.ResourceManager
	Owner   ssa.Owner
}

// GetOwnerLabels returns the inventory storage common labels.
func (m *Storage) GetOwnerLabels() client.MatchingLabels {
	return client.MatchingLabels{
		componentLabelKey: InventoryKindName,
		createdByLabelKey: m.Owner.Field,
	}
}

// CreateNamespace creates the inventory namespace if not present.
func (m *Storage) CreateNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				createdByLabelKey: m.Owner.Field,
			},
		},
	}

	if err := m.Manager.Client().Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
		if apierrors.IsNotFound(err) {
			opts := []client.PatchOption{
				client.ForceOwnership,
				client.FieldOwner(m.Owner.Field),
			}
			return m.Manager.Client().Patch(ctx, ns, client.Apply, opts...)
		} else {
			return err
		}
	}

	return nil
}

// ApplyInventory creates or updates the storage object for the given inventory.
func (m *Storage) ApplyInventory(ctx context.Context, i *Inventory) error {
	data, err := json.Marshal(i.Entries)
	if err != nil {
		return err
	}

	cm := m.newConfigMap(i.Name, i.Namespace)
	cm.Annotations = map[string]string{
		m.Owner.Group + "/last-applied-time": time.Now().UTC().Format(time.RFC3339),
	}
	if i.Source != "" {
		cm.Annotations[m.Owner.Group+"/source"] = i.Source
	}
	if i.Revision != "" {
		cm.Annotations[m.Owner.Group+"/revision"] = i.Revision
	}

	cm.Data = map[string]string{
		InventoryKindName: string(data),
	}

	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(m.Owner.Field),
	}
	return m.Manager.Client().Patch(ctx, cm, client.Apply, opts...)
}

// GetInventory retrieves the entries from the storage for the given inventory name and namespace.
func (m *Storage) GetInventory(ctx context.Context, i *Inventory) error {
	cm := m.newConfigMap(i.Name, i.Namespace)

	cmKey := client.ObjectKeyFromObject(cm)
	err := m.Manager.Client().Get(ctx, cmKey, cm)
	if err != nil {
		return err
	}

	if _, ok := cm.Data[InventoryKindName]; !ok {
		return fmt.Errorf("inventory data not found in ConfigMap/%s", cmKey)
	}

	var entries []Entry
	err = json.Unmarshal([]byte(cm.Data[InventoryKindName]), &entries)
	if err != nil {
		return err
	}

	i.Entries = entries

	for k, v := range cm.GetAnnotations() {
		switch k {
		case m.Owner.Group + "/source":
			i.Source = v
		case m.Owner.Group + "/revision":
			i.Revision = v
		}
	}

	return nil
}

// DeleteInventory removes the storage for the given inventory name and namespace.
func (m *Storage) DeleteInventory(ctx context.Context, i *Inventory) error {
	cm := m.newConfigMap(i.Name, i.Namespace)

	cmKey := client.ObjectKeyFromObject(cm)
	err := m.Manager.Client().Delete(ctx, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ConfigMap/%s, error: %w", cmKey, err)
	}
	return nil
}

// GetInventoryStaleObjects returns the list of objects metadata subject to pruning.
func (m *Storage) GetInventoryStaleObjects(ctx context.Context, i *Inventory) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	existingInventory := NewInventory(i.Name, i.Namespace)
	if err := m.GetInventory(ctx, existingInventory); err != nil {
		if apierrors.IsNotFound(err) {
			return objects, nil
		}
		return nil, err
	}

	objects, err := existingInventory.Diff(i)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (m *Storage) newConfigMap(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      InventoryPrefix + name,
			Namespace: namespace,
			Labels: map[string]string{
				nameLabelKey:      name,
				componentLabelKey: InventoryKindName,
				createdByLabelKey: m.Owner.Field,
			},
		},
	}
}
