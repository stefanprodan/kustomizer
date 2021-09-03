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
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/inventory"
	"github.com/stefanprodan/kustomizer/pkg/manager"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply Kubernetes manifests and Kustomize overlays using server-side apply.",
	RunE:  runApplyCmd,
}

type applyFlags struct {
	filename           []string
	kustomize          string
	inventoryName      string
	inventoryNamespace string
	wait               bool
	force              bool
	prune              bool
	mode               string
}

var applyArgs applyFlags

func init() {
	applyCmd.Flags().StringSliceVarP(&applyArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	applyCmd.Flags().StringVarP(&applyArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	applyCmd.Flags().BoolVar(&applyArgs.wait, "wait", false, "Wait for the applied Kubernetes objects to become ready.")
	applyCmd.Flags().BoolVar(&applyArgs.force, "force", false, "Recreate objects that contain immutable fields changes.")
	applyCmd.Flags().BoolVar(&applyArgs.prune, "prune", false, "Delete stale objects from the cluster.")
	applyCmd.Flags().StringVarP(&applyArgs.inventoryName, "inventory-name", "i", "", "The name of the inventory configmap.")
	applyCmd.Flags().StringVar(&applyArgs.inventoryNamespace, "inventory-namespace", "default",
		"The namespace of the inventory configmap. The namespace must exist on the target cluster.")
	applyCmd.Flags().StringVar(&applyArgs.mode, "mode", "Apply",
		"The ResourceManager apply method, can be `Apply`, `ApplyAll`, `ApplyAllStaged`.")
	rootCmd.AddCommand(applyCmd)
}

func runApplyCmd(cmd *cobra.Command, args []string) error {
	if applyArgs.kustomize == "" && len(applyArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}
	if applyArgs.inventoryName == "" {
		return fmt.Errorf("--inventory-name is required")
	}
	if applyArgs.inventoryNamespace == "" {
		return fmt.Errorf("--inventory-namespace is required")
	}

	logger.Println("building inventory...")
	objects, err := buildManifests(applyArgs.kustomize, applyArgs.filename)
	if err != nil {
		return err
	}

	newInventory := inventory.NewInventory(applyArgs.inventoryName, applyArgs.inventoryNamespace)
	if err := newInventory.AddObjects(objects); err != nil {
		return fmt.Errorf("creating inventory failed, error: %w", err)
	}
	logger.Println(fmt.Sprintf("applying %v manifest(s)...", len(objects)))

	kubeClient, err := newKubeClient(rootArgs.kubeconfig, rootArgs.kubecontext)
	if err != nil {
		return fmt.Errorf("client init failed: %w", err)
	}

	statusPoller, err := newKubeStatusPoller(rootArgs.kubeconfig, rootArgs.kubecontext)
	if err != nil {
		return fmt.Errorf("status poller init failed: %w", err)
	}

	resMgr := manager.NewResourceManager(kubeClient, statusPoller, manager.Owner{
		Field: PROJECT,
		Group: PROJECT + ".dev",
	})

	resMgr.SetOwnerLabels(objects, applyArgs.inventoryName, applyArgs.inventoryNamespace)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	switch applyArgs.mode {
	case "Apply":
		for _, object := range objects {
			change, err := resMgr.Apply(ctx, object, applyArgs.force)
			if err != nil {
				return err
			}
			logger.Println(change.String())
		}
	case "ApplyAll":
		changeSet, err := resMgr.ApplyAll(ctx, objects, applyArgs.force)
		if err != nil {
			return err
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}
	case "ApplyAllStaged":
		changeSet, err := resMgr.ApplyAllStaged(ctx, objects, applyArgs.force, 30*time.Second)
		if err != nil {
			return err
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}
	default:
		return fmt.Errorf("mode not supported")
	}

	staleObjects, err := inventoryMgr.GetStaleObjects(ctx, resMgr.KubeClient(), newInventory, applyArgs.inventoryName, applyArgs.inventoryNamespace)
	if err != nil {
		return fmt.Errorf("inventory query failed, error: %w", err)
	}

	err = inventoryMgr.Store(ctx, resMgr.KubeClient(), newInventory, applyArgs.inventoryName, applyArgs.inventoryNamespace)
	if err != nil {
		return fmt.Errorf("inventory apply failed, error: %w", err)
	}

	if applyArgs.prune && len(staleObjects) > 0 {
		changeSet, err := resMgr.DeleteAll(ctx, staleObjects)
		if err != nil {
			return fmt.Errorf("prune failed, error: %w", err)
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}
	}

	if applyArgs.wait {
		logger.Println("waiting for resources to become ready...")

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

		logger.Println("all resources are ready")
	}

	return nil
}
