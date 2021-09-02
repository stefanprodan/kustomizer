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

package resmgr

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Apply performs a server-side apply of the given object if the matching in-cluster object is different or if it doesn't exist.
// Drift detection is performed by comparing the server-side dry-run result with the existing object.
// When immutable field changes are detected, the object is recreated if 'force' is set to 'true'.
func (kc *ResourceManager) Apply(ctx context.Context, object *unstructured.Unstructured, force bool) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	_ = kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	dryRunObject := object.DeepCopy()
	if err := kc.dryRunApply(ctx, dryRunObject); err != nil {
		if force && strings.Contains(err.Error(), "immutable") {
			if err := kc.kubeClient.Delete(ctx, existingObject); err != nil {
				return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
					kc.fmt.Unstructured(dryRunObject), err)
			}
			return kc.Apply(ctx, object, force)
		}

		return nil, kc.validationError(dryRunObject, err)
	}

	// do not apply objects that have not drifted to avoid bumping the resource version
	if !kc.hasDrifted(existingObject, dryRunObject) {
		return kc.changeSetEntry(object, UnchangedAction), nil
	}

	appliedObject := object.DeepCopy()
	if err := kc.apply(ctx, appliedObject); err != nil {
		return nil, fmt.Errorf("%s apply failed, error: %w", kc.fmt.Unstructured(appliedObject), err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return kc.changeSetEntry(appliedObject, CreatedAction), nil
	}

	return kc.changeSetEntry(appliedObject, ConfiguredAction), nil
}

// ApplyAll performs a server-side dry-run of the given objects, and based on the diff result,
// it applies the objects that are new or modified.
func (kc *ResourceManager) ApplyAll(ctx context.Context, objects []*unstructured.Unstructured, force bool) (*ChangeSet, error) {
	sort.Sort(ApplyOrder(objects))
	changeSet := NewChangeSet()
	var toApply []*unstructured.Unstructured
	for _, object := range objects {
		existingObject := object.DeepCopy()
		_ = kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

		dryRunObject := object.DeepCopy()
		if err := kc.dryRunApply(ctx, dryRunObject); err != nil {
			if force && strings.Contains(err.Error(), "immutable") {
				if err := kc.kubeClient.Delete(ctx, existingObject); err != nil {
					return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
						kc.fmt.Unstructured(dryRunObject), err)
				}
				return kc.ApplyAll(ctx, objects, force)
			}

			return nil, kc.validationError(dryRunObject, err)
		}

		if kc.hasDrifted(existingObject, dryRunObject) {
			toApply = append(toApply, object)
			if dryRunObject.GetResourceVersion() == "" {
				changeSet.Add(*kc.changeSetEntry(dryRunObject, CreatedAction))
			} else {
				changeSet.Add(*kc.changeSetEntry(dryRunObject, ConfiguredAction))
			}
		} else {
			changeSet.Add(*kc.changeSetEntry(dryRunObject, UnchangedAction))
		}
	}

	for _, object := range toApply {
		appliedObject := object.DeepCopy()
		if err := kc.apply(ctx, appliedObject); err != nil {
			return nil, fmt.Errorf("%s apply failed, error: %w", kc.fmt.Unstructured(appliedObject), err)
		}
	}

	return changeSet, nil
}

// ApplyAllStaged extracts the CRDs and Namespaces, applies them with ApplyAll,
// waits for CRDs and Namespaces to become ready, then is applies all the other objects.
// This function should be used when the given objects have a mix of custom resource definition and custom resources,
// or a mix of namespace definitions with namespaced objects.
func (kc *ResourceManager) ApplyAllStaged(ctx context.Context, objects []*unstructured.Unstructured, force bool, wait time.Duration) (*ChangeSet, error) {
	changeSet := NewChangeSet()

	// contains only CRDs and Namespaces
	var stageOne []*unstructured.Unstructured

	// contains all objects except for  CRDs and Namespaces
	var stageTwo []*unstructured.Unstructured

	for _, u := range objects {
		if IsClusterDefinition(u.GetKind()) {
			stageOne = append(stageOne, u)
		} else {
			stageTwo = append(stageTwo, u)
		}
	}

	if len(stageOne) > 0 {
		cs, err := kc.ApplyAll(ctx, stageOne, force)
		if err != nil {
			return nil, err
		}
		changeSet.Append(cs.Entries)

		if err := kc.Wait(stageOne, 2*time.Second, wait); err != nil {
			return nil, err
		}
	}

	cs, err := kc.ApplyAll(ctx, stageTwo, force)
	if err != nil {
		return nil, err
	}
	changeSet.Append(cs.Entries)

	return changeSet, nil
}

func (kc *ResourceManager) dryRunApply(ctx context.Context, object *unstructured.Unstructured) error {
	opts := []client.PatchOption{
		client.DryRunAll,
		client.ForceOwnership,
		client.FieldOwner(kc.fieldOwner),
	}
	return kc.kubeClient.Patch(ctx, object, client.Apply, opts...)
}

func (kc *ResourceManager) apply(ctx context.Context, object *unstructured.Unstructured) error {
	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(kc.fieldOwner),
	}
	return kc.kubeClient.Patch(ctx, object, client.Apply, opts...)
}
