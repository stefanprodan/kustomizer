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
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InventoryManager records the Kubernetes objects that are applied on the cluster.
type InventoryManager struct {
	fieldOwner string
	group      string
}

// NewInventoryManager returns an InventoryManager.
func NewInventoryManager(fieldOwner, group string) (*InventoryManager, error) {
	if fieldOwner == "" {
		return nil, fmt.Errorf("fieldOwner is required")
	}
	if group == "" {
		return nil, fmt.Errorf("group is required")
	}

	return &InventoryManager{
		fieldOwner: fieldOwner,
		group:      group,
	}, nil
}

// Store applies the Inventory object on the server.
func (im *InventoryManager) Store(ctx context.Context, kubeClient client.Client, inv *Inventory, name, namespace string) error {
	data, err := json.Marshal(inv.Entries)
	if err != nil {
		return err
	}

	cm := im.newConfigMap(name, namespace)
	cm.Annotations = map[string]string{
		im.group + "/last-applied-time": time.Now().UTC().Format(time.RFC3339),
	}
	cm.Data = map[string]string{
		"inventory": string(data),
	}

	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(im.fieldOwner),
	}
	return kubeClient.Patch(ctx, cm, client.Apply, opts...)
}

// Retrieve fetches the Inventory object from the server.
func (im *InventoryManager) Retrieve(ctx context.Context, kubeClient client.Client, name, namespace string) (*Inventory, error) {
	cm := im.newConfigMap(name, namespace)

	cmKey := client.ObjectKeyFromObject(cm)
	err := kubeClient.Get(ctx, cmKey, cm)
	if err != nil {
		return nil, err
	}

	if _, ok := cm.Data["inventory"]; !ok {
		return nil, fmt.Errorf("inventory data not found in ConfigMap/%s", cmKey)
	}

	var entries []Entry
	err = json.Unmarshal([]byte(cm.Data["inventory"]), &entries)
	if err != nil {
		return nil, err
	}

	return &Inventory{Entries: entries}, nil
}

// GetStaleObjects returns the list of objects subject to pruning.
func (im *InventoryManager) GetStaleObjects(ctx context.Context, kubeClient client.Client, inv *Inventory, name, namespace string) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	exInv, err := im.Retrieve(ctx, kubeClient, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return objects, nil
		}
		return nil, err
	}

	objects, err = exInv.Diff(inv)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

// Remove deletes the Inventory object from the server.
func (im *InventoryManager) Remove(ctx context.Context, kubeClient client.Client, name, namespace string) error {
	cm := im.newConfigMap(name, namespace)

	cmKey := client.ObjectKeyFromObject(cm)
	err := kubeClient.Delete(ctx, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ConfigMap/%s, error: %w", cmKey, err)
	}
	return nil
}

func (im *InventoryManager) newConfigMap(name, namespace string) *corev1.ConfigMap {
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
				"app.kubernetes.io/component":  "inventory",
				"app.kubernetes.io/created-by": im.fieldOwner,
			},
		},
	}
}
