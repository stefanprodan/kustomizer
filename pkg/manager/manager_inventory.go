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

package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stefanprodan/kustomizer/pkg/inventory"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const inventoryKindName = "inventory"

// ApplyInventory creates or updates the ConfigMap object for the given inventory.
func (m *ResourceManager) ApplyInventory(ctx context.Context, i *inventory.Inventory) error {
	data, err := json.Marshal(i.Entries)
	if err != nil {
		return err
	}

	cm := m.newConfigMap(i.Name, i.Namespace)
	cm.Annotations = map[string]string{
		m.owner.Group + "/last-applied-time": time.Now().UTC().Format(time.RFC3339),
	}
	cm.Data = map[string]string{
		inventoryKindName: string(data),
	}

	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(m.owner.Field),
	}
	return m.client.Patch(ctx, cm, client.Apply, opts...)
}

// GetInventory retrieves the entries from the ConfigMap for the given inventory name and namespace.
func (m *ResourceManager) GetInventory(ctx context.Context, i *inventory.Inventory) error {
	cm := m.newConfigMap(i.Name, i.Namespace)

	cmKey := client.ObjectKeyFromObject(cm)
	err := m.client.Get(ctx, cmKey, cm)
	if err != nil {
		return err
	}

	if _, ok := cm.Data[inventoryKindName]; !ok {
		return fmt.Errorf("inventory data not found in ConfigMap/%s", cmKey)
	}

	var entries []inventory.Entry
	err = json.Unmarshal([]byte(cm.Data[inventoryKindName]), &entries)
	if err != nil {
		return err
	}

	i.Entries = entries
	return nil
}

// DeleteInventory removes the ConfigMap for the given inventory name and namespace.
func (m *ResourceManager) DeleteInventory(ctx context.Context, i *inventory.Inventory) error {
	cm := m.newConfigMap(i.Name, i.Namespace)

	cmKey := client.ObjectKeyFromObject(cm)
	err := m.client.Delete(ctx, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ConfigMap/%s, error: %w", cmKey, err)
	}
	return nil
}

// GetInventoryStaleObjects returns the list of objects subject to pruning.
func (m *ResourceManager) GetInventoryStaleObjects(ctx context.Context, i *inventory.Inventory) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	existingInventory := inventory.NewInventory(i.Name, i.Namespace)
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

func (m *ResourceManager) newConfigMap(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       name,
				"app.kubernetes.io/component":  inventoryKindName,
				"app.kubernetes.io/created-by": m.owner.Field,
			},
		},
	}
}
