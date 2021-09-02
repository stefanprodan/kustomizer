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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
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

// KubeClient returns the underlying controller-runtime client.
func (kc *ResourceManager) KubeClient() client.Client {
	return kc.kubeClient
}

func (kc *ResourceManager) changeSetEntry(object *unstructured.Unstructured, action Action) *ChangeSetEntry {
	return &ChangeSetEntry{Subject: kc.fmt.Unstructured(object), Action: string(action)}
}
