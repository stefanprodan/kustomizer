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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ApplyOrder implements the Sort interface for unstructured objects.
type ApplyOrder []*unstructured.Unstructured

func (objects ApplyOrder) Len() int {
	return len(objects)
}

func (objects ApplyOrder) Swap(i, j int) {
	objects[i], objects[j] = objects[j], objects[i]
}

func (objects ApplyOrder) Less(i, j int) bool {
	ki := objects[i].GetKind()
	ni := objects[i].GetName()
	kj := objects[j].GetKind()
	nj := objects[j].GetName()
	ranki, rankj := rankOfKind(ki), rankOfKind(kj)
	if ranki == rankj {
		return ni < nj
	}
	return ranki < rankj
}

// rankOfKind returns an int denoting the position of the given kind
// in the partial ordering of Kubernetes resources, according to which
// kinds depend on which (derived by hand).
func rankOfKind(kind string) int {
	switch strings.ToLower(kind) {
	// API extensions
	case "customresourcedefinition":
		return 0
	// Global objects
	case "namespace", "clusterrolebinding", "clusterrole":
		return 1
	// Namespaced objects
	case "serviceaccount", "role", "rolebinding", "service", "endpoint", "ingress":
		return 2
	// Namespaced objects
	case "resourcequota", "limitrange", "secret", "configmap", "persistentvolume", "persistentvolumeclaim":
		return 2
	// Workload objects
	case "daemonset", "deployment", "job", "cronjob", "statefulset", "replicationcontroller", "replicaset", "pod":
		return 3
	default:
		return 4
	}
}
