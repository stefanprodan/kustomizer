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
	"sort"

	"github.com/stefanprodan/kustomizer/pkg/objectutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/ordering"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Delete deletes the given object (not found errors are ignored).
func (m *ResourceManager) Delete(ctx context.Context, object *unstructured.Unstructured) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	err := m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%s query failed, error: %w", objectutil.FmtUnstructured(object), err)
		}
	} else {
		if err := m.client.Delete(ctx, existingObject); err != nil {
			return nil, fmt.Errorf("%s delete failed, error: %w", objectutil.FmtUnstructured(object), err)
		}
	}

	return m.changeSetEntry(object, DeletedAction), nil
}

// DeleteAll deletes the given set of objects (not found errors are ignored).
func (m *ResourceManager) DeleteAll(ctx context.Context, objects []*unstructured.Unstructured) (*ChangeSet, error) {
	sort.Sort(sort.Reverse(ordering.SortableUnstructureds(objects)))
	changeSet := NewChangeSet()

	for _, object := range objects {
		cse, err := m.Delete(ctx, object)
		if err != nil {
			return nil, err
		}
		changeSet.Add(*cse)
	}

	return changeSet, nil
}
