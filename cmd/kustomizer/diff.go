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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff compares the local Kubernetes manifests with the in-cluster objects and prints the YAML diff to stdout.",
	RunE:  runDiffCmd,
}

type diffFlags struct {
	filename           []string
	kustomize          string
	patch              []string
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
	diffCmd.Flags().StringSliceVarP(&diffArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")
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

	var objects []*unstructured.Unstructured

	if len(diffArgs.filename) == 1 && diffArgs.filename[0] == "-" {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		objs, err := ssa.ReadObjects(bytes.NewReader(data))
		if err != nil {
			return err
		}
		objects = objs
	} else {
		objs, err := buildManifests(diffArgs.kustomize, diffArgs.filename, diffArgs.patch)
		if err != nil {
			return err
		}
		objects = objs
	}

	newInventory := inventory.NewInventory(diffArgs.inventoryName, diffArgs.inventoryNamespace)
	if err := newInventory.AddObjects(objects); err != nil {
		return fmt.Errorf("creating inventory failed, error: %w", err)
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

	invStorage := &inventory.InventoryStorage{
		Manager: resMgr,
		Owner:   inventoryOwner,
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	if diffArgs.inventoryName != "" {
		resMgr.SetOwnerLabels(objects, diffArgs.inventoryName, diffArgs.inventoryNamespace)
	}

	if _, err := exec.LookPath("diff"); err != nil {
		return fmt.Errorf("diff binary not found in PATH, error: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", diffArgs.inventoryName)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	invalid := false
	for _, object := range objects {
		change, liveObject, mergedObject, err := resMgr.Diff(ctx, object)
		if err != nil {
			logger.Println(`✗`, err)
			invalid = true
			continue
		}

		if change.Action == string(ssa.CreatedAction) {
			fmt.Println(`►`, change.Subject, "created")
		}

		if change.Action == string(ssa.ConfiguredAction) {
			fmt.Println(`►`, change.Subject, "drifted")

			liveYAML, _ := yaml.Marshal(liveObject)
			liveFile := filepath.Join(tmpDir, "live.yaml")
			if err := os.WriteFile(liveFile, liveYAML, 0644); err != nil {
				return err
			}

			mergedYAML, _ := yaml.Marshal(mergedObject)
			mergedFile := filepath.Join(tmpDir, "merged.yaml")
			if err := os.WriteFile(mergedFile, mergedYAML, 0644); err != nil {
				return err
			}

			out, _ := exec.Command("diff", "-N", "-u", liveFile, mergedFile).Output()
			for i, line := range strings.Split(string(out), "\n") {
				if i > 1 && len(line) > 0 {
					fmt.Println(line)
				}
			}
		}
	}

	if diffArgs.inventoryName != "" {
		staleObjects, err := invStorage.GetInventoryStaleObjects(ctx, newInventory)
		if err != nil {
			return fmt.Errorf("inventory query failed, error: %w", err)
		}

		for _, object := range staleObjects {
			fmt.Println(`►`, fmt.Sprintf("%s deleted", ssa.FmtUnstructured(object)))
		}
	}

	if invalid {
		os.Exit(1)
	}
	return nil
}
