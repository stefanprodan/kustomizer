package resmgr

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var manager *ResourceManager

func TestMain(m *testing.M) {
	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}

	kubeClient, err := client.NewWithWatch(cfg, client.Options{Scheme: newScheme()})
	if err != nil {
		panic(err)
	}

	restMapper, err := apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		panic(err)
	}

	c, err := client.New(cfg, client.Options{Mapper: restMapper})
	if err != nil {
		panic(err)
	}

	poller := polling.NewStatusPoller(c, restMapper)

	manager = &ResourceManager{
		kubeClient:    kubeClient,
		kstatusPoller: poller,
		fmt:           &ResourceFormatter{},
		fieldOwner:    "resource-manager",
	}

	code := m.Run()

	testEnv.Stop()

	os.Exit(code)
}

func readManifest(manifest, namespace string) ([]*unstructured.Unstructured, error) {
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	yml := fmt.Sprintf(string(data), namespace)

	reader := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(yml), 2048)
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

func setNamespace(objects []*unstructured.Unstructured, namespace string) {
	for _, object := range objects {
		object.SetNamespace(namespace)
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "Namespace",
		Version: "v1",
	})
	u.SetName(namespace)
	objects = append(objects, u)
}

var nextNameId int64

func generateName(prefix string) string {
	id := atomic.AddInt64(&nextNameId, 1)
	return fmt.Sprintf("%s-%d", prefix, id)
}

func removeObject(s []*unstructured.Unstructured, index int) []*unstructured.Unstructured {
	return append(s[:index], s[index+1:]...)
}
