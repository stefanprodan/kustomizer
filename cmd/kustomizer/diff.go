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

	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/pkg/resmgr"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff compares the local Kubernetes manifests with the in-cluster objects and prints the YAML diff.",
	RunE:  runDiffCmd,
}

type diffFlags struct {
	filename           []string
	kustomize          string
	inventoryName      string
	inventoryNamespace string
	prune              bool
}

var diffArgs diffFlags

func init() {
	diffCmd.Flags().StringSliceVarP(&diffArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	diffCmd.Flags().StringVarP(&diffArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	diffCmd.Flags().BoolVar(&diffArgs.prune, "prune", false, "Delete stale objects from the cluster.")
	diffCmd.Flags().StringVarP(&diffArgs.inventoryName, "inventory-name", "i", "", "The name of the inventory configmap.")
	diffCmd.Flags().StringVar(&diffArgs.inventoryNamespace, "inventory-namespace", "default",
		"The namespace of the inventory configmap. The namespace must exist on the target cluster.")
	rootCmd.AddCommand(diffCmd)
}

func runDiffCmd(cmd *cobra.Command, args []string) error {
	if diffArgs.kustomize == "" && len(diffArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	objects, err := buildManifests(diffArgs.kustomize, diffArgs.filename)
	if err != nil {
		return err
	}

	newInventory, err := inventoryMgr.Record(objects)
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
		change, err := resMgr.Diff(ctx, object)
		if err != nil {
			return err
		}

		if change.Action == string(resmgr.CreatedAction) {
			logger.Println(`►`, change.Subject, "created")
		}

		if change.Action == string(resmgr.ConfiguredAction) {
			logger.Println(`►`, change.Subject, "drifted")
			logger.Println(change.Diff)
		}

	}

	if diffArgs.inventoryName != "" {
		staleObjects, err := inventoryMgr.GetStaleObjects(ctx, resMgr.KubeClient(), newInventory, diffArgs.inventoryName, diffArgs.inventoryNamespace)
		if err != nil {
			return fmt.Errorf("inventory query failed, error: %w", err)
		}

		for _, object := range staleObjects {
			logger.Println(`►`, fmt.Sprintf("%s deleted", resourceFormatter.Unstructured(object)))
		}
	}
	return nil
}
