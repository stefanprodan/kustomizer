package manager

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stefanprodan/kustomizer/pkg/inventory"
	"github.com/stefanprodan/kustomizer/pkg/objectutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func TestInventory(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := generateName("inventory")
	objects, err := readManifest("testdata/test1.yaml", id)
	if err != nil {
		t.Fatal(err)
	}

	manager.SetOwnerLabels(objects, "app1", "default")

	inv := inventory.NewInventory(id, "default")
	inv.SetSource("https://github.com/stefanprodan/kustomizer.git", "v1.0.0")

	t.Run("adds entries in the right order", func(t *testing.T) {
		if err := inv.AddObjects(objects); err != nil {
			t.Fatal(err)
		}

		// expected created order
		sort.Sort(objectutil.SortableUnstructureds(objects))
		var expected []string
		for _, object := range objects {
			expected = append(expected, objectutil.FmtUnstructured(object))
		}

		list, err := inv.ListMeta()
		if err != nil {
			t.Fatal(err)
		}

		var output []string
		for _, meta := range list {
			output = append(output, objectutil.FmtObjMetadata(meta))
		}

		if diff := cmp.Diff(expected, output); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})

	t.Run("contains the right API version", func(t *testing.T) {
		_, role := getFirstObject(objects, "ClusterRole", id)
		roleMeta := object.UnstructuredToObjMeta(role)

		found := false
		for _, entry := range inv.Entries {
			if entry.ObjectID == roleMeta.String() && entry.ObjectVersion == role.GroupVersionKind().Version {
				found = true
			}
		}

		if !found {
			t.Fatal("API version not found")
		}
	})

	t.Run("applies the ConfigMap", func(t *testing.T) {
		if err := manager.ApplyInventory(ctx, inv); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("reads the ConfigMap", func(t *testing.T) {
		res := inventory.NewInventory(id, "default")
		if err := manager.GetInventory(ctx, res); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(res.Source, inv.Source); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(res.Revision, inv.Revision); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		if err := manager.GetInventory(ctx, inventory.NewInventory("x", "default")); !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
	})

	t.Run("detects stale objects", func(t *testing.T) {
		empty, err := manager.GetInventoryStaleObjects(ctx, inv)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(len(empty), 0); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}

		newInv := inventory.NewInventory(id, "default")
		var newObjs []*unstructured.Unstructured

		for _, u := range objects {
			if u.GetKind() != "ConfigMap" {
				newObjs = append(newObjs, u)
			}
		}

		if err := newInv.AddObjects(newObjs); err != nil {
			t.Fatal(err)
		}

		stale, err := manager.GetInventoryStaleObjects(ctx, newInv)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(stale[0].GetKind(), "ConfigMap"); diff != "" {
			t.Errorf("Mismatch from expected value (-want +got):\n%s", diff)
		}
	})
}
