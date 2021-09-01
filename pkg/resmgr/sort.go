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

// ApplyOrder implements the Sort interface for Kubernetes objects based on kind.
// When creating objects: CRDs, namespaces and other global kinds go first, while webhooks go last.
// When deleting objects: the order is inverted to allow the Kubernetes controllers to finalize custom resources.
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
	ranki, rankj := RankOfKind(ki), RankOfKind(kj)
	if ranki == rankj {
		return ni < nj
	}
	return ranki < rankj
}

// RankOfKind returns an int denoting the position of the given kind in the partial ordering of Kubernetes resources.
func RankOfKind(kind string) int {
	switch strings.ToLower(kind) {
	case "customresourcedefinition", "apiservice":
		return 0
	case "namespace":
		return 1
	case "clusterrole", "clusterrolebinding", "ingressclass", "runtimeclass", "storageclass", "priorityclass", "certificatesigningrequest", "podsecuritypolicy":
		return 2
	case "secret", "configmap", "lease", "serviceaccount", "role", "rolebinding", "service", "endpoint", "endpointslice", "ingress", "networkpolicy":
		return 3
	case "resourcequota", "limitrange", "podpreset", "persistentvolume", "persistentvolumeclaim", "poddisruptionbudget", "horizontalpodautoscaler":
		return 4
	case "daemonset", "deployment", "job", "cronjob", "statefulset", "replicationcontroller", "replicaset", "pod":
		return 5
	default:
		return 6
	case "mutatingwebhookconfiguration", "validatingwebhookconfiguration":
		return 7
	}
}
