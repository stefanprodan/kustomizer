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
	"sigs.k8s.io/cli-utils/pkg/object"
)

const fmtSeparator = "/"

type ResourceFormatter struct {
	Separator string
}

func (rf *ResourceFormatter) ObjMetadata(obj object.ObjMetadata) string {
	var builder strings.Builder
	builder.WriteString(obj.GroupKind.Kind + rf.getSeparator())
	if obj.Namespace != "" {
		builder.WriteString(obj.Namespace + rf.getSeparator())
	}
	builder.WriteString(obj.Name)
	return builder.String()
}

func (rf *ResourceFormatter) Unstructured(obj *unstructured.Unstructured) string {
	return rf.ObjMetadata(object.UnstructuredToObjMeta(obj))
}

func (rf *ResourceFormatter) getSeparator() string {
	if rf.Separator == "" {
		return fmtSeparator
	}

	return rf.Separator
}
