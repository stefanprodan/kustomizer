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

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
	"github.com/stefanprodan/kustomizer/pkg/resmgr"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply Kubernetes manifests and Kustomize overlays using server-side apply.",
	RunE:  runApplyCmd,
}

type applyFlags struct {
	filename           []string
	kustomize          string
	output             string
	inventoryName      string
	inventoryNamespace string
	wait               bool
	force              bool
	prune              bool
}

var applyArgs applyFlags

func init() {
	applyCmd.Flags().StringSliceVarP(&applyArgs.filename, "filename", "f", nil, "path to Kubernetes manifest(s)")
	applyCmd.Flags().StringVarP(&applyArgs.kustomize, "kustomize", "k", "", "process a kustomization directory (can't be used together with -f)")
	applyCmd.Flags().BoolVar(&applyArgs.wait, "wait", false, "wait for the applied Kubernetes objects to become ready")
	applyCmd.Flags().BoolVar(&applyArgs.force, "force", false, "recreate objects that contain immutable fields changes")
	applyCmd.Flags().BoolVar(&applyArgs.prune, "prune", false, "delete stale objects")
	applyCmd.Flags().StringVarP(&applyArgs.output, "output", "o", "", "output can be yaml or json")
	applyCmd.Flags().StringVarP(&applyArgs.inventoryName, "inventory-name", "i", "", "inventory configmap name")
	applyCmd.Flags().StringVar(&applyArgs.inventoryNamespace, "inventory-namespace", "default", "inventory configmap namespace")

	rootCmd.AddCommand(applyCmd)
}

func runApplyCmd(cmd *cobra.Command, args []string) error {
	invMgr := inventory.NewInventoryManager(PROJECT)
	objects := make([]*unstructured.Unstructured, 0)

	if applyArgs.kustomize != "" {
		data, err := buildKustomization(applyArgs.kustomize)
		if err != nil {
			return err
		}

		objs, err := invMgr.ReadAll(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", applyArgs.kustomize, err)
		}
		objects = append(objects, objs...)
	} else {
		if len(applyArgs.filename) == 0 {
			return fmt.Errorf("-f or -k is required")
		}

		manifests, err := scan(applyArgs.filename)
		if err != nil {
			return err
		}
		for _, manifest := range manifests {
			ms, err := os.Open(manifest)
			if err != nil {
				return err
			}

			objs, err := invMgr.ReadAll(bufio.NewReader(ms))
			ms.Close()
			if err != nil {
				return fmt.Errorf("%s: %w", manifest, err)
			}
			objects = append(objects, objs...)
		}
	}

	sort.Sort(resmgr.ApplyOrder(objects))
	if applyArgs.output != "" {
		switch applyArgs.output {
		case "yaml":
			yml, err := invMgr.ToYAML(objects)
			if err != nil {
				return err
			}
			fmt.Println(yml)
			return nil
		case "json":
			json, err := invMgr.ToJSON(objects)
			if err != nil {
				return err
			}
			fmt.Println(json)
			return nil
		default:
			return fmt.Errorf("unsupported output, can be yaml or json")
		}
	}

	if applyArgs.inventoryName == "" {
		return fmt.Errorf("--inventory-name is required")
	}
	if applyArgs.inventoryNamespace == "" {
		return fmt.Errorf("--inventory-namespace is required")
	}

	newInventory, err := invMgr.Record(objects)
	if err != nil {
		return fmt.Errorf("creating inventory failed, error: %w", err)
	}

	resMgr, err := resmgr.NewResourceManager(rootArgs.kubeconfig, rootArgs.kubecontext, PROJECT)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	for _, object := range objects {
		change, err := resMgr.Apply(ctx, object, applyArgs.force)
		if err != nil {
			return err
		}
		fmt.Println(change.String())
	}

	staleObjects, err := invMgr.GetStaleObjects(ctx, resMgr.KubeClient(), newInventory, applyArgs.inventoryName, applyArgs.inventoryNamespace)
	if err != nil {
		return fmt.Errorf("inventory query failed, error: %w", err)
	}

	err = invMgr.Store(ctx, resMgr.KubeClient(), newInventory, applyArgs.inventoryName, applyArgs.inventoryNamespace)
	if err != nil {
		return fmt.Errorf("inventory apply failed, error: %w", err)
	}

	if applyArgs.prune && len(staleObjects) > 0 {
		changeSet, err := resMgr.DeleteAll(ctx, staleObjects)
		if err != nil {
			return fmt.Errorf("prune failed, error: %w", err)
		}
		for _, change := range changeSet.Entries {
			fmt.Println(change.String())
		}
	}

	if applyArgs.wait {
		fmt.Println("waiting for resources to become ready...")

		err = resMgr.Wait(objects, 2*time.Second, rootArgs.timeout)
		if err != nil {
			return err
		}

		if applyArgs.prune && len(staleObjects) > 0 {
			err = resMgr.WaitForTermination(staleObjects, 2*time.Second, rootArgs.timeout)
			if err != nil {
				return fmt.Errorf("wating for termination failed, error: %w", err)
			}
		}

		fmt.Println("all resources are ready")
	}

	return nil
}
