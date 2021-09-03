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

// NewResourceManager creates a ResourceManager for the given Kubernetes client config and context.
func NewResourceManager(client client.Client, poller *polling.StatusPoller, owner Owner) *ResourceManager {
	return &ResourceManager{
		client: client,
		poller: poller,
		owner:  owner,
	}
}

// KubeClient returns the underlying controller-runtime client.
func (kc *ResourceManager) KubeClient() client.Client {
	return kc.client
}

func (kc *ResourceManager) changeSetEntry(object *unstructured.Unstructured, action Action) *ChangeSetEntry {
	return &ChangeSetEntry{Subject: objectutil.FmtUnstructured(object), Action: string(action)}
}

//func (kc *ResourceManager) SetOwnerLabels(objects []*unstructured.Unstructured, name, namespace string) {
//	for _, object := range objects {
//		object.SetLabels(map[string]string{
//			kc.fieldOwner + "/name":      name,
//			kc.fieldOwner + "/namespace": namespace,
//		})
//	}
//}
