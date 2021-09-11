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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stefanprodan/kustomizer/pkg/objectutil"
)

// ResourceManager reconciles Kubernetes resources onto the target cluster.
type ResourceManager struct {
	client client.Client
	poller *polling.StatusPoller
	owner  Owner
}

// NewResourceManager creates a ResourceManager for the given Kubernetes client.
func NewResourceManager(client client.Client, poller *polling.StatusPoller, owner Owner) *ResourceManager {
	return &ResourceManager{
		client: client,
		poller: poller,
		owner:  owner,
	}
}

// Client returns the underlying controller-runtime client.
func (m *ResourceManager) Client() client.Client {
	return m.client
}

// SetOwnerLabels adds the ownership labels to the given objects.
// The ownership labels are in the format:
// 	<owner.group>/name: <name>
// 	<owner.group>/namespace: <namespace>
func (m *ResourceManager) SetOwnerLabels(objects []*unstructured.Unstructured, name, namespace string) {
	for _, object := range objects {
		labels := object.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}

		labels[m.owner.Group+"/name"] = name
		labels[m.owner.Group+"/namespace"] = namespace

		object.SetLabels(labels)
	}
}

func (m *ResourceManager) changeSetEntry(object *unstructured.Unstructured, action Action) *ChangeSetEntry {
	return &ChangeSetEntry{Subject: objectutil.FmtUnstructured(object), Action: string(action)}
}
