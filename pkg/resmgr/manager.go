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
	"github.com/google/go-cmp/cmp"
	"sort"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/aggregator"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/collector"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// Diff performs a server-side apply dry-un and returns the fields that changed.
func (kc *ResourceManager) Diff(ctx context.Context, object *unstructured.Unstructured) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	_ = kc.kubeClient.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	dryRunObject := object.DeepCopy()
	if err := kc.dryRunApply(ctx, dryRunObject); err != nil {
		return nil, fmt.Errorf("%s apply dry-run failed, error: %w", kc.fmt.Unstructured(dryRunObject), err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return kc.changeSetEntry(dryRunObject, CreatedAction, ""), nil
	}

	// do not apply objects that have not drifted to avoid bumping the resource version
	if drift, diff := kc.hasDrifted(existingObject, dryRunObject); drift {
		return kc.changeSetEntry(object, ConfiguredAction, diff), nil
	}

	return kc.changeSetEntry(dryRunObject, UnchangedAction, ""), nil
}

// Apply performs a server-side apply of the given object if the matching in-cluster object is different or if it doesn't exist.
// Drift detection is performed by comparing the server-side dry-run result with the existing object.
// When immutable field changes are detected, the object is recreated if 'force' is set to 'true'.
func (kc *ResourceManager) Apply(ctx context.Context, object *unstructured.Unstructured, force bool) (*ChangeSetEntry, error) {
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
				return kc.Apply(ctx, object, force)
			}
		}
		return nil, fmt.Errorf("%s apply dry-run failed, error: %w", kc.fmt.Unstructured(dryRunObject), err)
	}

	// do not apply objects that have not drifted to avoid bumping the resource version
	if drift, _ := kc.hasDrifted(existingObject, dryRunObject); !drift {
		return kc.changeSetEntry(object, UnchangedAction, ""), nil
	}

	appliedObject := object.DeepCopy()
	if err := kc.apply(ctx, appliedObject); err != nil {
		return nil, fmt.Errorf("%s apply failed, error: %w", kc.fmt.Unstructured(appliedObject), err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return kc.changeSetEntry(appliedObject, CreatedAction, ""), nil
	}
	return kc.changeSetEntry(appliedObject, ConfiguredAction, ""), nil
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
			if _, ok := apierrors.StatusCause(err, metav1.CauseTypeFieldValueInvalid); ok {
				if force && strings.Contains(err.Error(), "immutable") {
					if err := kc.kubeClient.Delete(ctx, existingObject); err != nil {
						return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
							kc.fmt.Unstructured(dryRunObject), err)
					}
					return kc.ApplyAll(ctx, objects, force)
				}
			}
			return nil, fmt.Errorf("%s apply dry-run failed, error: %w", kc.fmt.Unstructured(dryRunObject), err)
		}

		if drift, _ := kc.hasDrifted(existingObject, dryRunObject); drift {
			toApply = append(toApply, object)
			if dryRunObject.GetResourceVersion() == "" {
				changeSet.Add(*kc.changeSetEntry(dryRunObject, CreatedAction, ""))
			} else {
				changeSet.Add(*kc.changeSetEntry(dryRunObject, ConfiguredAction, ""))
			}
		} else {
			changeSet.Add(*kc.changeSetEntry(dryRunObject, UnchangedAction, ""))
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
		changeSet.AddAll(cs.Entries)

		if err := kc.Wait(stageOne, 2*time.Second, wait); err != nil {
			return nil, err
		}
	}

	cs, err := kc.ApplyAll(ctx, stageTwo, force)
	if err != nil {
		return nil, err
	}
	changeSet.AddAll(cs.Entries)

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
				changeSet.Add(*kc.changeSetEntry(object, DeletedAction, ""))
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
func (kc *ResourceManager) hasDrifted(existingObject, dryRunObject *unstructured.Unstructured) (bool, string) {
	if dryRunObject.GetResourceVersion() == "" {
		return true, ""
	}

	if !apiequality.Semantic.DeepDerivative(dryRunObject.GetLabels(), existingObject.GetLabels()) {
		return true, cmp.Diff(dryRunObject.Object, existingObject.Object)

	}

	if !apiequality.Semantic.DeepDerivative(dryRunObject.GetAnnotations(), existingObject.GetAnnotations()) {
		return true, cmp.Diff(dryRunObject.Object, existingObject.Object)
	}

	if _, ok := existingObject.Object["spec"]; ok {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object["spec"], existingObject.Object["spec"]) {
			return true, cmp.Diff(dryRunObject.Object, existingObject.Object)
		}
	} else if _, ok := existingObject.Object["webhooks"]; ok {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object["webhooks"], existingObject.Object["webhooks"]) {
			return true, cmp.Diff(dryRunObject.Object, existingObject.Object)
		}
	} else {
		if !apiequality.Semantic.DeepDerivative(dryRunObject.Object, existingObject.Object) {
			return true, cmp.Diff(dryRunObject.Object, existingObject.Object)
		}
	}

	return false, ""
}

func (kc *ResourceManager) changeSetEntry(object *unstructured.Unstructured, action Action, diff string) *ChangeSetEntry {
	return &ChangeSetEntry{kc.fmt.Unstructured(object), string(action), diff}
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

// KubeClient returns the underlying controller-runtime client.
func (kc *ResourceManager) KubeClient() client.Client {
	return kc.kubeClient
}
