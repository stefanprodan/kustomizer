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
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// InventoryManager records the Kubernetes objects that are applied on the cluster.
type InventoryManager struct {
	fieldOwner string
}

// NewInventoryManager returns an InventoryManager.
func NewInventoryManager(fieldOwner string) *InventoryManager {
	return &InventoryManager{fieldOwner: fieldOwner}
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

// Store applies the Inventory object on the server.
func (im *InventoryManager) Store(ctx context.Context, kubeClient client.Client, inv *Inventory, name, namespace string) error {
	data, err := json.Marshal(inv.Entries)
	if err != nil {
		return err
	}

	cm := im.newConfigMap(name, namespace)
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

	var entries map[string]string
	err = json.Unmarshal([]byte(cm.Data["inventory"]), &entries)
	if err != nil {
		return nil, err
	}

	return &Inventory{Entries: entries}, nil
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

// Record creates an Inventory of the given objects.
func (im *InventoryManager) Record(objects []*unstructured.Unstructured) (*Inventory, error) {
	inventory := NewInventory()

	if err := inventory.Add(objects); err != nil {
		return nil, err
	}

	return inventory, nil
}

// Read decodes a YAML or JSON document from the given reader into an unstructured Kubernetes API object.
func (im *InventoryManager) Read(r io.Reader) (*unstructured.Unstructured, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	obj := &unstructured.Unstructured{}
	err := reader.Decode(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// ReadAll decodes the YAML or JSON documents from the given reader into unstructured Kubernetes API objects.
func (im *InventoryManager) ReadAll(r io.Reader) ([]*unstructured.Unstructured, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	objects := make([]*unstructured.Unstructured, 0)

	for {
		obj := &unstructured.Unstructured{}
		err := reader.Decode(obj)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return objects, err
		}

		if obj.IsList() {
			err = obj.EachListItem(func(item apiruntime.Object) error {
				obj := item.(*unstructured.Unstructured)
				objects = append(objects, obj)
				return nil
			})
			if err != nil {
				return objects, err
			}
			continue
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// ToYAML encodes the given Kubernetes API objects to a YAML multi-doc.
func (im *InventoryManager) ToYAML(objects []*unstructured.Unstructured) (string, error) {
	var builder strings.Builder
	for _, obj := range objects {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return "", err
		}
		builder.Write(data)
		builder.WriteString("---\n")
	}
	return builder.String(), nil
}

// ToJSON encodes the given Kubernetes API objects to a YAML multi-doc.
func (im *InventoryManager) ToJSON(objects []*unstructured.Unstructured) (string, error) {
	list := struct {
		ApiVersion string                       `json:"apiVersion,omitempty"`
		Kind       string                       `json:"kind,omitempty"`
		Items      []*unstructured.Unstructured `json:"items,omitempty"`
	}{
		ApiVersion: "v1",
		Kind:       "ListMeta",
		Items:      objects,
	}

	data, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		return "", err
	}

	return string(data), nil
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
		},
	}
}
