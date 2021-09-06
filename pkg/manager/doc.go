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

// Package manager contains utilities for managing Kubernetes resources.
//
// The ResourceManager can be used to write a GitOps reconciler that:
// - maintains an inventory of objects applied on the cluster
// - orders the Kubernetes objects for apply (CRDs, Namespaces first, Webhooks last)
// - validates the objects with server-side dry-run apply
// - determines if the in-cluster objects are in drift based on the dry-run result
// - applies the objects on the cluster with server-side apply
// - waits for the objects to be fully reconciled by looking up their readiness status
// - deletes objects that are subject to garbage collection
// - waits for the deleted objects to be terminated
package manager
