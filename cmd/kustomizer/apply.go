/*
Copyright 2021 Stefan Prodan

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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply validates the given Kubernetes manifests and Kustomize overlays and reconciles them using server-side apply.",
	RunE:  runApplyCmd,
}

type applyFlags struct {
	filename           []string
	kustomize          string
	patch              []string
	inventoryName      string
	inventoryNamespace string
	wait               bool
	force              bool
	prune              bool
	source             string
	revision           string
	artifact           []string
}

var applyArgs applyFlags

func init() {
	applyCmd.Flags().StringSliceVarP(&applyArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	applyCmd.Flags().StringVarP(&applyArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	applyCmd.Flags().StringSliceVarP(&applyArgs.artifact, "artifact", "a", nil,
		"Image URL in the format 'oci://domain/org/repo:tag' e.g. 'oci://docker.io/stefanprodan/app-deploy:v1.0.0'.")
	applyCmd.Flags().StringSliceVarP(&applyArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")
	applyCmd.Flags().BoolVar(&applyArgs.wait, "wait", false, "Wait for the applied Kubernetes objects to become ready.")
	applyCmd.Flags().BoolVar(&applyArgs.force, "force", false, "Recreate objects that contain immutable fields changes.")
	applyCmd.Flags().BoolVar(&applyArgs.prune, "prune", false, "Delete stale objects from the cluster.")
	applyCmd.Flags().StringVarP(&applyArgs.inventoryName, "inventory-name", "i", "", "The name of the inventory configmap.")
	applyCmd.Flags().StringVar(&applyArgs.inventoryNamespace, "inventory-namespace", "default",
		"The namespace of the inventory configmap. The namespace must exist on the target cluster.")
	applyCmd.Flags().StringVar(&applyArgs.source, "source", "", "The URL to the source code.")
	applyCmd.Flags().StringVar(&applyArgs.revision, "revision", "", "The revision identifier.")

	rootCmd.AddCommand(applyCmd)
}

func runApplyCmd(cmd *cobra.Command, args []string) error {
	if applyArgs.kustomize == "" && len(applyArgs.filename) == 0 && len(applyArgs.artifact) == 0 {
		return fmt.Errorf("-f, -k or -a is required")
	}
	if applyArgs.inventoryName == "" {
		return fmt.Errorf("--inventory-name is required")
	}
	if applyArgs.inventoryNamespace == "" {
		return fmt.Errorf("--inventory-namespace is required")
	}

	var objects []*unstructured.Unstructured

	switch {
	case len(applyArgs.filename) == 1 && applyArgs.filename[0] == "-":
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		objs, err := ssa.ReadObjects(bytes.NewReader(data))
		if err != nil {
			return err
		}
		objects = objs
	case len(applyArgs.artifact) > 0:
		tmpDir, err := os.MkdirTemp("", "oci")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
		defer cancel()
		for i, ociURL := range applyArgs.artifact {
			url, err := registry.ParseURL(ociURL)
			if err != nil {
				return err
			}

			logger.Println("pulling", url)
			yml, err := registry.Pull(ctx, url)
			if err != nil {
				return fmt.Errorf("pulling %s failed: %w", ociURL, err)
			}

			if err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("%v.yaml", i)), []byte(yml), 0666); err != nil {
				return fmt.Errorf("extracting manifests from' %s' failed: %w", ociURL, err)
			}
		}
		objs, err := buildManifests("", []string{tmpDir}, applyArgs.patch)
		if err != nil {
			return err
		}
		objects = objs
	default:
		logger.Println("building inventory...")
		objs, err := buildManifests(applyArgs.kustomize, applyArgs.filename, applyArgs.patch)
		if err != nil {
			return err
		}
		objects = objs
	}

	newInventory := inventory.NewInventory(applyArgs.inventoryName, applyArgs.inventoryNamespace)
	newInventory.SetSource(applyArgs.source, applyArgs.revision)
	if err := newInventory.AddObjects(objects); err != nil {
		return fmt.Errorf("creating inventory failed, error: %w", err)
	}
	logger.Println(fmt.Sprintf("applying %v manifest(s)...", len(objects)))

	for _, object := range objects {
		fixReplicasConflict(object, objects)
	}

	kubeClient, err := newKubeClient(kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("client init failed: %w", err)
	}

	statusPoller, err := newKubeStatusPoller(kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("status poller init failed: %w", err)
	}

	resMgr := ssa.NewResourceManager(kubeClient, statusPoller, inventoryOwner)
	resMgr.SetOwnerLabels(objects, applyArgs.inventoryName, applyArgs.inventoryNamespace)

	invStorage := &inventory.InventoryStorage{
		Manager: resMgr,
		Owner:   inventoryOwner,
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// contains only CRDs and Namespaces
	var stageOne []*unstructured.Unstructured

	// contains all objects except for CRDs and Namespaces
	var stageTwo []*unstructured.Unstructured

	for _, u := range objects {
		if ssa.IsClusterDefinition(u) {
			stageOne = append(stageOne, u)
		} else {
			stageTwo = append(stageTwo, u)
		}
	}

	applyOpts := ssa.DefaultApplyOptions()
	applyOpts.Force = applyArgs.force

	waitOpts := ssa.DefaultWaitOptions()
	waitOpts.Timeout = rootArgs.timeout

	if len(stageOne) > 0 {
		changeSet, err := resMgr.ApplyAll(ctx, stageOne, applyOpts)
		if err != nil {
			return err
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}

		if err := resMgr.Wait(stageOne, waitOpts); err != nil {
			return err
		}
	}

	sort.Sort(ssa.SortableUnstructureds(stageTwo))
	for _, object := range stageTwo {
		change, err := resMgr.Apply(ctx, object, applyOpts)
		if err != nil {
			return err
		}
		logger.Println(change.String())
	}

	staleObjects, err := invStorage.GetInventoryStaleObjects(ctx, newInventory)
	if err != nil {
		return fmt.Errorf("inventory query failed, error: %w", err)
	}

	err = invStorage.ApplyInventory(ctx, newInventory)
	if err != nil {
		return fmt.Errorf("inventory apply failed, error: %w", err)
	}

	if applyArgs.prune && len(staleObjects) > 0 {
		changeSet, err := resMgr.DeleteAll(ctx, staleObjects, ssa.DefaultDeleteOptions())
		if err != nil {
			return fmt.Errorf("prune failed, error: %w", err)
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}
	}

	if applyArgs.wait {
		logger.Println("waiting for resources to become ready...")

		err = resMgr.Wait(objects, waitOpts)
		if err != nil {
			return err
		}

		if applyArgs.prune && len(staleObjects) > 0 {

			err = resMgr.WaitForTermination(staleObjects, waitOpts)
			if err != nil {
				return fmt.Errorf("wating for termination failed, error: %w", err)
			}
		}

		logger.Println("all resources are ready")
	}

	return nil
}

// fixReplicasConflict removes the replicas field from the given workload if it's managed by an HPA
func fixReplicasConflict(object *unstructured.Unstructured, objects []*unstructured.Unstructured) {
	for _, hpa := range objects {
		if hpa.GetKind() == "HorizontalPodAutoscaler" && object.GetNamespace() == hpa.GetNamespace() {
			targetKind, found, err := unstructured.NestedFieldCopy(hpa.Object, "spec", "scaleTargetRef", "kind")
			if err == nil && found && fmt.Sprintf("%v", targetKind) == object.GetKind() {
				targetName, found, err := unstructured.NestedFieldCopy(hpa.Object, "spec", "scaleTargetRef", "name")
				if err == nil && found && fmt.Sprintf("%v", targetName) == object.GetName() {
					unstructured.RemoveNestedField(object.Object, "spec", "replicas")
				}
			}
		}
	}
}
