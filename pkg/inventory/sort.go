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

package inventory

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// InventoryOrder implements the Sort interface for unstructured objects.
type InventoryOrder []*unstructured.Unstructured

func (objects InventoryOrder) Len() int {
	return len(objects)
}

func (objects InventoryOrder) Swap(i, j int) {
	objects[i], objects[j] = objects[j], objects[i]
}

func (objects InventoryOrder) Less(i, j int) bool {
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
// in the partial ordering of Kubernetes resources.
func rankOfKind(kind string) int {
	switch strings.ToLower(kind) {
	// API extensions
	case "customresourcedefinition":
		return 0
	// Namespace objects
	case "namespace":
		return 1
	// Global objects
	case "clusterrole", "clusterrolebinding", "ingressclass", "storageclass":
		return 2
	default:
		return 3
	}
}
