package manager

import (
	"context"
	"github.com/stefanprodan/kustomizer/pkg/objectutil"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDelete(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("delete")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	_, configMap := getFirstObject(objects, "ConfigMap", id)
	_, role := getFirstObject(objects, "ClusterRole", id)

	if _, err = manager.ApplyAllStaged(ctx, objects, false, timeout); err != nil {
		t.Fatal(err)
	}

	t.Run("deletes objects in order", func(t *testing.T) {
		changeSet, err := manager.DeleteAll(ctx, objects)
		if err != nil {
			t.Fatal(err)
		}

		// expected deleted order
		var expected []string
		for _, object := range objects {
			expected = append(expected, objectutil.FmtUnstructured(object))
		}

		// verify the change set contains only created actions
		var output []string
		for _, entry := range changeSet.Entries {
			if diff := cmp.Diff(entry.Action, string(DeletedAction)); diff != "" {
				t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
			}
			output = append(output, entry.Subject)
		}

		// verify the change set contains all objects in the right order
		if diff := cmp.Diff(expected, output); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		configMapClone := configMap.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(configMapClone), configMapClone)
		if !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}

		roleClone := role.DeepCopy()
		err = manager.client.Get(ctx, client.ObjectKeyFromObject(roleClone), roleClone)
		if !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
	})

	t.Run("waits for objects termination", func(t *testing.T) {
		_, err := manager.DeleteAll(ctx, objects)
		if err != nil {
			t.Fatal(err)
		}

		if err := manager.WaitForTermination(objects, time.Second, 5*time.Second); err != nil {
			// workaround for https://github.com/kubernetes-sigs/controller-runtime/issues/880
			if !strings.Contains(err.Error(), "Namespace/") {
				t.Fatal(err)
			}
		}
	})
}
