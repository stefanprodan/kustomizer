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
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ResourceManager reconciles Kubernetes resources onto the target cluster.
type ResourceManager struct {
	kubeClient    client.WithWatch
	kstatusPoller *polling.StatusPoller
	fmt           *ResourceFormatter
	fieldOwner    string
}

// NewResourceManager creates a ResourceManager for the given Kubernetes client config and context.
func NewResourceManager(kubeConfigPath, kubeContext, fieldOwner string) (*ResourceManager, error) {
	kubeClient, err := newKubeClient(kubeConfigPath, kubeContext)
	if err != nil {
		return nil, fmt.Errorf("client init failed: %w", err)
	}

	statusPoller, err := newKubeStatusPoller(kubeConfigPath, kubeContext)
	if err != nil {
		return nil, fmt.Errorf("status poller init failed: %w", err)
	}

	return &ResourceManager{
		kubeClient:    kubeClient,
		kstatusPoller: statusPoller,
		fmt:           &ResourceFormatter{},
		fieldOwner:    fieldOwner,
	}, nil
}

// Read decodes a YAML or JSON document from the given reader into an unstructured Kubernetes API object.
func (kc *ResourceManager) Read(r io.Reader) (*unstructured.Unstructured, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	obj := &unstructured.Unstructured{}
	err := reader.Decode(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// ReadAll decodes the YAML or JSON documents from the given reader into unstructured Kubernetes API objects.
func (kc *ResourceManager) ReadAll(r io.Reader) ([]*unstructured.Unstructured, error) {
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

	sort.Sort(ApplyOrder(objects))
	return objects, nil
}

// Reconcile performs a server-side apply of the given object if the matching in-cluster object is different or if it doesn't exist.
// Drift detection is performed by comparing the server-side dry-run result with the existing object.
// When immutable field changes are detected, the object is recreated if 'force' is set to 'true'.
func (kc *ResourceManager) Reconcile(ctx context.Context, object *unstructured.Unstructured, force bool) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	_ = kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	dryRunObject := object.DeepCopy()
	if err := kc.dryRunApply(ctx, dryRunObject); err != nil {
		if _, ok := apierrors.StatusCause(err, metav1.CauseTypeFieldValueInvalid); ok {
			if force && strings.Contains(err.Error(), "immutable") {
				if err := kc.kubeClient.Delete(ctx, existingObject); err != nil {
					return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
						kc.fmt.Unstructured(dryRunObject), err)
				}
				return kc.Reconcile(ctx, object, force)
			}
		}
		return nil, fmt.Errorf("%s apply dry-run failed, error: %w", kc.fmt.Unstructured(dryRunObject), err)
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

// ReconcileAll performs a server-side apply of the given set of objects.
func (kc *ResourceManager) ReconcileAll(ctx context.Context, objects []*unstructured.Unstructured, force bool) (*ChangeSet, error) {
	sort.Sort(ApplyOrder(objects))
	changeSet := NewChangeSet()
	var toApply []*unstructured.Unstructured
	for _, object := range objects {
		existingObject := object.DeepCopy()
		_ = kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

		dryRunObject := object.DeepCopy()
		if err := kc.dryRunApply(ctx, dryRunObject); err != nil {
			if _, ok := apierrors.StatusCause(err, metav1.CauseTypeFieldValueInvalid); ok {
				if force && strings.Contains(err.Error(), "immutable") {
					if err := kc.kubeClient.Delete(ctx, existingObject); err != nil {
						return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
							kc.fmt.Unstructured(dryRunObject), err)
					}
					return kc.ReconcileAll(ctx, objects, force)
				}
			}
			return nil, fmt.Errorf("%s apply dry-run failed, error: %w", kc.fmt.Unstructured(dryRunObject), err)
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

// DeleteAll deletes the given set of objects.
func (kc *ResourceManager) DeleteAll(ctx context.Context, objects []*unstructured.Unstructured) (*ChangeSet, error) {
	sort.Sort(sort.Reverse(ApplyOrder(objects)))
	changeSet := NewChangeSet()

	for _, object := range objects {
		existingObject := object.DeepCopy()
		err := kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(object), existingObject)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("%s query failed, error: %w", kc.fmt.Unstructured(object), err)
			}
		} else {
			err := kc.kubeClient.Delete(ctx, existingObject)
			if err != nil {
				return nil, fmt.Errorf("%s delete failed, error: %w", kc.fmt.Unstructured(object), err)
			} else {
				changeSet.Add(*kc.changeSetEntry(object, DeletedAction))
			}
		}
	}
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

// hasDrifted detects changes to metadata labels, metadata annotations and spec.
func (kc *ResourceManager) hasDrifted(existingObject, dryRunObject *unstructured.Unstructured) bool {
	if dryRunObject.GetResourceVersion() == "" || existingObject == nil {
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
	} else {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object, existingObject.Object) {
			return true
		}
	}

	return false
}

func (kc *ResourceManager) changeSetEntry(object *unstructured.Unstructured, action Action) *ChangeSetEntry {
	return &ChangeSetEntry{kc.fmt.Unstructured(object), string(action)}
}

// Wait checks if the given set of objects has been fully reconciled.
func (kc *ResourceManager) Wait(objects []*unstructured.Unstructured, interval, timeout time.Duration) error {
	objectsMeta := object.UnstructuredsToObjMetas(objects)
	statusCollector := collector.NewResourceStatusCollector(objectsMeta)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := polling.Options{
		PollInterval: interval,
		UseCache:     true,
	}
	eventsChan := kc.kstatusPoller.Poll(ctx, objectsMeta, opts)

	lastStatus := make(map[object.ObjMetadata]*event.ResourceStatus)

	done := statusCollector.ListenWithObserver(eventsChan, collector.ObserverFunc(
		func(statusCollector *collector.ResourceStatusCollector, e event.Event) {
			var rss []*event.ResourceStatus
			for _, rs := range statusCollector.ResourceStatuses {
				if rs == nil {
					continue
				}
				if rs.Error == nil {
					lastStatus[rs.Identifier] = rs
				}
				rss = append(rss, rs)
			}
			desired := status.CurrentStatus
			aggStatus := aggregator.AggregateStatus(rss, desired)
			if aggStatus == desired {
				cancel()
				return
			}
		}),
	)

	<-done

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	if ctx.Err() == context.DeadlineExceeded {
		var errors = []string{}
		for id, rs := range statusCollector.ResourceStatuses {
			if rs == nil {
				errors = append(errors, fmt.Sprintf("can't determine status for %s", kc.fmt.ObjMetadata(id)))
				continue
			}
			if lastStatus[id].Status != status.CurrentStatus {
				var builder strings.Builder
				builder.WriteString(fmt.Sprintf("%s status: '%s'",
					kc.fmt.ObjMetadata(rs.Identifier), lastStatus[id].Status))
				if rs.Error != nil {
					builder.WriteString(fmt.Sprintf(": %s", rs.Error))
				}
				errors = append(errors, builder.String())
			}
		}
		return fmt.Errorf("timeout waiting for: [%s]", strings.Join(errors, ", "))
	}

	return nil
}

// WaitForTermination waits for the given objects to be deleted from the cluster.
func (kc *ResourceManager) WaitForTermination(objects []*unstructured.Unstructured, interval, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, object := range objects {
		if err := wait.PollImmediate(interval, timeout, kc.isDeleted(ctx, object)); err != nil {
			return err
		}
	}
	return nil
}

func (kc *ResourceManager) isDeleted(ctx context.Context, object *unstructured.Unstructured) wait.ConditionFunc {
	return func() (bool, error) {
		obj := object.DeepCopy()
		err := kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
}

// ToYAML encodes the given Kubernetes API objects to a YAML multi-doc.
func (kc *ResourceManager) ToYAML(objects []*unstructured.Unstructured) (string, error) {
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
func (kc *ResourceManager) ToJSON(objects []*unstructured.Unstructured) (string, error) {
	list := struct {
		ApiVersion string                       `json:"apiVersion,omitempty"`
		Kind       string                       `json:"kind,omitempty"`
		Items      []*unstructured.Unstructured `json:"items,omitempty"`
	}{
		ApiVersion: "v1",
		Kind:       "List",
		Items:      objects,
	}

	data, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
