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
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func BenchmarkTestApply10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timeout := 10 * time.Second

		id := generateName("bench")
		objects, err := readManifest("testdata/test1.yaml", id)
		if err != nil {
			panic(err)
		}

		_, err = manager.ApplyAllStaged(context.Background(), objects, false, timeout)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkTestApplyWait10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timeout := 10 * time.Second

		id := generateName("bench")
		objects, err := readManifest("testdata/test1.yaml", id)
		if err != nil {
			b.Fatal(err)
		}

		if _, err = manager.ApplyAllStaged(context.Background(), objects, false, timeout); err != nil {
			b.Fatal(err)
		}

		if err := manager.Wait(objects, time.Second, 5*time.Second); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkTestApplyDelete10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timeout := 10 * time.Second
		id := generateName("bench")
		objects, err := readManifest("testdata/test1.yaml", id)
		if err != nil {
			b.Fatal(err)
		}

		if _, err = manager.ApplyAllStaged(context.Background(), objects, false, timeout); err != nil {
			b.Fatal(err)
		}

		if _, err := manager.DeleteAll(context.Background(), objects); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkTestApplyDeleteWait10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timeout := 10 * time.Second
		id := generateName("bench")
		objects, err := readManifest("testdata/test1.yaml", id)
		if err != nil {
			b.Fatal(err)
		}

		if _, err = manager.ApplyAllStaged(context.Background(), objects, false, timeout); err != nil {
			b.Fatal(err)
		}

		if _, err := manager.DeleteAll(context.Background(), objects); err != nil {
			b.Error(err)
		}

		_, configmap := getFirstObject(objects, "ConfigMap", id)
		_, role := getFirstObject(objects, "ClusterRole", id)
		if err := manager.WaitForTermination([]*unstructured.Unstructured{role, configmap}, time.Second, timeout); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkTestApplyDryRun10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		id := generateName("bench")
		objects, err := readManifest("testdata/test1.yaml", id)
		if err != nil {
			panic(err)
		}

		_, role := getFirstObject(objects, "ClusterRole", id)
		if _, err := manager.Diff(context.Background(), role); err != nil {
			b.Error(err)
		}

		_, roleb := getFirstObject(objects, "ClusterRoleBinding", id)
		if _, err := manager.Diff(context.Background(), roleb); err != nil {
			b.Error(err)
		}

		_, ns := getFirstObject(objects, "Namespace", id)
		if _, err := manager.Diff(context.Background(), ns); err != nil {
			b.Error(err)
		}
	}
}
