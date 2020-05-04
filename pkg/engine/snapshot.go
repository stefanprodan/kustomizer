package engine

import (
	"bytes"
	js "encoding/json"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type Snapshot struct {
	Revision string          `json:"revision"`
	Entries  []SnapshotEntry `json:"entries"`
}

type SnapshotEntry struct {
	Namespace string            `json:"namespace"`
	Kinds     map[string]string `json:"kinds"`
}

func NewSnapshot(manifests []byte, revision string) (*Snapshot, error) {
	snapshot := Snapshot{
		Revision: revision,
		Entries:  []SnapshotEntry{},
	}

	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifests), 2048)
	for {
		var obj unstructured.Unstructured
		err := reader.Decode(&obj)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if obj.IsList() {
			err := obj.EachListItem(func(item runtime.Object) error {
				snapshot.addEntry(item.(*unstructured.Unstructured))
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			snapshot.addEntry(&obj)
		}
	}

	return &snapshot, nil
}

func NewSnapshotFromConfigMap(manifest string) (*Snapshot, error) {
	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(manifest), 2048)
	var cm corev1.ConfigMap
	err := reader.Decode(&cm)
	if err != nil {
		return nil, err
	}

	if _, ok := cm.Data["snapshot"]; !ok {
		return nil, fmt.Errorf("snapshot data not found")
	}

	data := []byte(cm.Data["snapshot"])

	var snapshot Snapshot
	err = js.Unmarshal(data, &snapshot)
	if err != nil {
		return nil, err
	}

	return &snapshot, err
}

func (s *Snapshot) ToConfigMap(name, namespace string) (string, error) {
	data, err := js.Marshal(s)
	if err != nil {
		return "", err
	}
	cm := &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"snapshot": string(data),
		},
	}

	scheme := runtime.NewScheme()
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{
		Pretty: false,
		Yaml:   false,
		Strict: true,
	})

	buf := bytes.NewBufferString("")
	err = serializer.Encode(cm, buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *Snapshot) addEntry(item *unstructured.Unstructured) {
	found := false
	for _, tracker := range s.Entries {
		if tracker.Namespace == item.GetNamespace() {
			tracker.Kinds[item.GetKind()] = item.GetAPIVersion()
			found = true
			break
		}
	}
	if !found {
		s.Entries = append(s.Entries, SnapshotEntry{
			Namespace: item.GetNamespace(),
			Kinds: map[string]string{
				item.GetKind(): item.GetAPIVersion(),
			},
		})
	}
}

func (s *Snapshot) NonNamespacedKinds() []string {
	kinds := make([]string, 0)
	for _, tracker := range s.Entries {
		if tracker.Namespace == "" {
			for k, _ := range tracker.Kinds {
				kinds = append(kinds, k)
			}
		}
	}
	return kinds
}

func (s *Snapshot) NamespacedKinds() map[string][]string {
	nsk := make(map[string][]string)
	for _, tracker := range s.Entries {
		if tracker.Namespace != "" {
			var kinds []string
			for k, _ := range tracker.Kinds {
				kinds = append(kinds, k)
			}
			nsk[tracker.Namespace] = kinds
		}
	}
	return nsk
}
