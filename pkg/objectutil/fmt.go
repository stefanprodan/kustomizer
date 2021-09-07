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

package objectutil

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const fmtSeparator = "/"

// FmtObjMetadata returns the object ID in the format <kind>/<namespace>/<name>.
func FmtObjMetadata(obj object.ObjMetadata) string {
	var builder strings.Builder
	builder.WriteString(obj.GroupKind.Kind + fmtSeparator)
	if obj.Namespace != "" {
		builder.WriteString(obj.Namespace + fmtSeparator)
	}
	builder.WriteString(obj.Name)
	return builder.String()
}

// FmtUnstructured returns the object ID in the format <kind>/<namespace>/<name>.
func FmtUnstructured(obj *unstructured.Unstructured) string {
	return FmtObjMetadata(object.UnstructuredToObjMeta(obj))
}

// MaskSecret replaces the data key values with the given mask.
func MaskSecret(object *unstructured.Unstructured, mask string) (*unstructured.Unstructured, error) {
	data, found, err := unstructured.NestedMap(object.Object, "data")
	if err != nil {
		return nil, err
	}

	if found {
		for k, _ := range data {
			data[k] = mask
		}

		err = unstructured.SetNestedMap(object.Object, data, "data")
		if err != nil {
			return nil, err
		}
	}

	return object, err
}
