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
	"fmt"
	"github.com/stefanprodan/kustomizer/pkg/objectutil"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Apply performs a server-side apply of the given object if the matching in-cluster object is different or if it doesn't exist.
// Drift detection is performed by comparing the server-side dry-run result with the existing object.
// When immutable field changes are detected, the object is recreated if 'force' is set to 'true'.
func (m *ResourceManager) Apply(ctx context.Context, object *unstructured.Unstructured, force bool) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	dryRunObject := object.DeepCopy()
	if err := m.dryRunApply(ctx, dryRunObject); err != nil {
		if force && strings.Contains(err.Error(), "immutable") {
			if err := m.client.Delete(ctx, existingObject); err != nil {
				return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
					objectutil.FmtUnstructured(dryRunObject), err)
			}
			return m.Apply(ctx, object, force)
		}

		return nil, m.validationError(dryRunObject, err)
	}

	// do not apply objects that have not drifted to avoid bumping the resource version
	if !m.hasDrifted(existingObject, dryRunObject) {
		return m.changeSetEntry(object, UnchangedAction), nil
	}

	appliedObject := object.DeepCopy()
	if err := m.apply(ctx, appliedObject); err != nil {
		return nil, fmt.Errorf("%s apply failed, error: %w", objectutil.FmtUnstructured(appliedObject), err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return m.changeSetEntry(appliedObject, CreatedAction), nil
	}

	return m.changeSetEntry(appliedObject, ConfiguredAction), nil
}

// ApplyAll performs a server-side dry-run of the given objects, and based on the diff result,
// it applies the objects that are new or modified.
func (m *ResourceManager) ApplyAll(ctx context.Context, objects []*unstructured.Unstructured, force bool) (*ChangeSet, error) {
	sort.Sort(objectutil.ApplyOrder(objects))
	changeSet := NewChangeSet()
	var toApply []*unstructured.Unstructured
	for _, object := range objects {
		existingObject := object.DeepCopy()
		_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

		dryRunObject := object.DeepCopy()
		if err := m.dryRunApply(ctx, dryRunObject); err != nil {
			if force && strings.Contains(err.Error(), "immutable") {
				if err := m.client.Delete(ctx, existingObject); err != nil {
					return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
						objectutil.FmtUnstructured(dryRunObject), err)
				}
				return m.ApplyAll(ctx, objects, force)
			}

			return nil, m.validationError(dryRunObject, err)
		}

		if m.hasDrifted(existingObject, dryRunObject) {
			toApply = append(toApply, object)
			if dryRunObject.GetResourceVersion() == "" {
				changeSet.Add(*m.changeSetEntry(dryRunObject, CreatedAction))
			} else {
				changeSet.Add(*m.changeSetEntry(dryRunObject, ConfiguredAction))
			}
		} else {
			changeSet.Add(*m.changeSetEntry(dryRunObject, UnchangedAction))
		}
	}

	for _, object := range toApply {
		appliedObject := object.DeepCopy()
		if err := m.apply(ctx, appliedObject); err != nil {
			return nil, fmt.Errorf("%s apply failed, error: %w", objectutil.FmtUnstructured(appliedObject), err)
		}
	}

	return changeSet, nil
}

// ApplyAllStaged extracts the CRDs and Namespaces, applies them with ApplyAll,
// waits for CRDs and Namespaces to become ready, then is applies all the other objects.
// This function should be used when the given objects have a mix of custom resource definition and custom resources,
// or a mix of namespace definitions with namespaced objects.
func (m *ResourceManager) ApplyAllStaged(ctx context.Context, objects []*unstructured.Unstructured, force bool, wait time.Duration) (*ChangeSet, error) {
	changeSet := NewChangeSet()

	// contains only CRDs and Namespaces
	var stageOne []*unstructured.Unstructured

	// contains all objects except for  CRDs and Namespaces
	var stageTwo []*unstructured.Unstructured

	for _, u := range objects {
		if objectutil.IsClusterDefinition(u.GetKind()) {
			stageOne = append(stageOne, u)
		} else {
			stageTwo = append(stageTwo, u)
		}
	}

	if len(stageOne) > 0 {
		cs, err := m.ApplyAll(ctx, stageOne, force)
		if err != nil {
			return nil, err
		}
		changeSet.Append(cs.Entries)

		if err := m.Wait(stageOne, 2*time.Second, wait); err != nil {
			return nil, err
		}
	}

	cs, err := m.ApplyAll(ctx, stageTwo, force)
	if err != nil {
		return nil, err
	}
	changeSet.Append(cs.Entries)

	return changeSet, nil
}

func (m *ResourceManager) dryRunApply(ctx context.Context, object *unstructured.Unstructured) error {
	opts := []client.PatchOption{
		client.DryRunAll,
		client.ForceOwnership,
		client.FieldOwner(m.owner.Field),
	}
	return m.client.Patch(ctx, object, client.Apply, opts...)
}

func (m *ResourceManager) apply(ctx context.Context, object *unstructured.Unstructured) error {
	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(m.owner.Field),
	}
	return m.client.Patch(ctx, object, client.Apply, opts...)
}
