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
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/stefanprodan/kustomizer/pkg/objectutil"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// Diff performs a server-side apply dry-un and returns the fields that changed in YAML format.
// If the diff contains Kubernetes Secrets, the data values are masked.
func (m *ResourceManager) Diff(ctx context.Context, object *unstructured.Unstructured) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	dryRunObject := object.DeepCopy()
	if err := m.dryRunApply(ctx, dryRunObject); err != nil {
		return nil, m.validationError(dryRunObject, err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return m.changeSetEntry(dryRunObject, CreatedAction), nil
	}

	if m.hasDrifted(existingObject, dryRunObject) {
		cse := m.changeSetEntry(object, ConfiguredAction)

		unstructured.RemoveNestedField(dryRunObject.Object, "metadata", "managedFields")
		unstructured.RemoveNestedField(existingObject.Object, "metadata", "managedFields")

		if dryRunObject.GetKind() == "Secret" {
			d, err := objectutil.MaskSecret(dryRunObject, "******")
			if err != nil {
				return nil, fmt.Errorf("masking secret data failed, error: %w", err)
			}
			dryRunObject = d
			ex, err := objectutil.MaskSecret(existingObject, "*****")
			if err != nil {
				return nil, fmt.Errorf("masking secret data failed, error: %w", err)
			}
			existingObject = ex
		}

		d, _ := yaml.Marshal(dryRunObject)
		e, _ := yaml.Marshal(existingObject)
		cse.Diff = cmp.Diff(string(e), string(d))

		return cse, nil
	}

	return m.changeSetEntry(dryRunObject, UnchangedAction), nil
}

// hasDrifted detects changes to metadata labels, metadata annotations, spec and webhooks.
func (m *ResourceManager) hasDrifted(existingObject, dryRunObject *unstructured.Unstructured) bool {
	if dryRunObject.GetResourceVersion() == "" {
		return true
	}

	if !apiequality.Semantic.DeepDerivative(dryRunObject.GetLabels(), existingObject.GetLabels()) {
		return true

	}

	if !apiequality.Semantic.DeepDerivative(dryRunObject.GetAnnotations(), existingObject.GetAnnotations()) {
		return true
	}

	if _, ok := existingObject.Object["spec"]; ok {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object["spec"], existingObject.Object["spec"]) {
			return true
		}
	} else if _, ok := existingObject.Object["webhooks"]; ok {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object["webhooks"], existingObject.Object["webhooks"]) {
			return true
		}
	} else {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object, existingObject.Object) {
			return true
		}
	}

	return false
}

// validationError formats the given error and hides sensitive data
// if the error was caused by an invalid Kubernetes secrets.
func (m *ResourceManager) validationError(object *unstructured.Unstructured, err error) error {
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("%s namespace not specified, error: %w", objectutil.FmtUnstructured(object), err)
	}

	reason := fmt.Sprintf("%v", apierrors.ReasonForError(err))

	if object.GetKind() == "Secret" {
		msg := "data values must be of type string"
		if strings.Contains(err.Error(), "immutable") {
			msg = "secret is immutable"
		}
		return fmt.Errorf("%s %s, error: %s", objectutil.FmtUnstructured(object), strings.ToLower(reason), msg)
	}

	// detect managed field conflict
	if status, ok := apierrors.StatusCause(err, metav1.CauseTypeFieldManagerConflict); ok {
		reason = fmt.Sprintf("%v", status.Type)
	}

	return fmt.Errorf("%s dry-run falied, reason: %s, error: %w",
		objectutil.FmtUnstructured(object), reason, err)

}
