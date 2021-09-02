package resmgr

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApply(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	objects, err := readManifest("testdata/test1.yaml", generateName("ns"))
	if err != nil {
		t.Fatal(err)
	}

	// create objects
	createdChangeSet, err := manager.ApplyAllStaged(ctx, objects, false, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// expected created order
	sort.Sort(ApplyOrder(objects))
	var expected []string
	for _, object := range objects {
		expected = append(expected, manager.fmt.Unstructured(object))
	}

	// verify the change set contains only created actions
	var output []string
	for _, entry := range createdChangeSet.Entries {
		if diff := cmp.Diff(entry.Action, string(CreatedAction)); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
		output = append(output, entry.Subject)
	}

	// verify the change set contains all objects in the right order
	if diff := cmp.Diff(expected, output); diff != "" {
		t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
	}

	// no-op apply
	unchangedChangeSet, err := manager.ApplyAllStaged(ctx, objects, false, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// verify the change set contains only unchanged actions
	for _, entry := range unchangedChangeSet.Entries {
		if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
		output = append(output, entry.Subject)
	}

	// extract configmap
	var configMap *unstructured.Unstructured
	for _, object := range objects {
		if object.GetKind() == "ConfigMap" {
			configMap = object
			break
		}
	}
	configMapName := manager.fmt.Unstructured(configMap)

	// update a value in the configmap
	err = unstructured.SetNestedField(configMap.Object, "val", "data", "key")
	if err != nil {
		t.Fatal(err)
	}

	// apply changes
	configuredChangeSet, err := manager.ApplyAllStaged(ctx, objects, false, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// verify the change set contains the configured action only for the configmap
	for _, entry := range configuredChangeSet.Entries {
		if entry.Subject == configMapName {
			if diff := cmp.Diff(string(ConfiguredAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		} else {
			if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	}

	// get the configmap from cluster
	configMapClone := configMap.DeepCopy()
	err = manager.kubeClient.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
	if err != nil {
		t.Fatal(err)
	}

	// get data value from the in-cluster configmap
	val, _, err := unstructured.NestedFieldCopy(configMapClone.Object, "data", "key")
	if err != nil {
		t.Fatal(err)
	}

	// verify the configmap was updated in cluster with the right data value
	if diff := cmp.Diff(val, "val"); diff != "" {
		t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
	}

	// delete the configmap
	deletedChangeSet, err := manager.DeleteAll(ctx, []*unstructured.Unstructured{configMap})
	for _, entry := range deletedChangeSet.Entries {
		if diff := cmp.Diff(string(DeletedAction), entry.Action); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	}

	// reapply objects
	changeSet, err := manager.ApplyAllStaged(ctx, objects, false, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// verify the configmap was recreated
	for _, entry := range changeSet.Entries {
		if entry.Subject == configMapName {
			if diff := cmp.Diff(string(CreatedAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		} else {
			if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	}

	// extract secret
	var secret *unstructured.Unstructured
	for _, object := range objects {
		if object.GetKind() == "Secret" {
			secret = object
			break
		}
	}
	secretName := manager.fmt.Unstructured(secret)

	// update a value in the secret
	err = unstructured.SetNestedField(secret.Object, "val-secret", "stringData", "key")
	if err != nil {
		t.Fatal(err)
	}

	// apply and expect to fail
	changeSet, err = manager.ApplyAllStaged(ctx, objects, false, timeout)
	if err == nil {
		t.Fatal("Expected error got none")
	}

	// verify that the error message does not contain sensitive information
	expectedErr := fmt.Sprintf("%s is invalid, error: secret is immutable", manager.fmt.Unstructured(secret))
	if diff := cmp.Diff(expectedErr, err.Error()); diff != "" {
		t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
	}

	// force apply
	changeSet, err = manager.ApplyAllStaged(ctx, objects, true, timeout)
	if err != nil {
		t.Fatal(err)
	}

	// verify the secret was recreated
	for _, entry := range changeSet.Entries {
		if entry.Subject == secretName {
			if diff := cmp.Diff(string(CreatedAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		} else {
			if diff := cmp.Diff(string(UnchangedAction), entry.Action); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
		}
	}

	// get the secret from cluster
	secretClone := secret.DeepCopy()
	err = manager.kubeClient.Get(ctx, client.ObjectKeyFromObject(secretClone), secretClone)
	if err != nil {
		t.Fatal(err)
	}

	// get data value from the in-cluster secret
	val, _, err = unstructured.NestedFieldCopy(secretClone.Object, "data", "key")
	if err != nil {
		t.Fatal(err)
	}

	// verify the secret was updated in cluster with the right data value
	if diff := cmp.Diff(val, base64.StdEncoding.EncodeToString([]byte("val-secret"))); diff != "" {
		t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
	}
}
