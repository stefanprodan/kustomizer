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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
)

var diffInventoryCmd = &cobra.Command{
	Use:     "inventory",
	Aliases: []string{"inv"},
	Short:   "Diff compares the given inventory with the in-cluster one and prints the YAML diff to stdout.",
	Example: `  kustomizer diff inventory <inv name> -n <inv namespace> [-a <oci url>] [-f <dir path>|<file path>] [-p <kustomize patch>] -k <overlay path>

  # Build the inventory from remote OCI artifacts and print the YAML diff
  kustomizer diff inventory my-app -n apps -a oci://registry/org/repo:latest

  # Build the inventory from remote OCI artifacts, apply local patches and print the YAML diff
  kustomizer diff inventory my-app -n apps -a oci://registry/org/repo:latest -p ./patches/safe-to-evict.yaml

  # Build the inventory from local files and print the YAML diff
  kustomizer diff inventory my-app -n apps -f ./deploy/manifests

  # Build the inventory from a local overlay and print the YAML diff
  kustomizer diff inventory my-app -n apps -k ./overlays/prod
`,
	RunE: runDiffInventoryCmd,
}

type diffInventoryFlags struct {
	artifact  []string
	filename  []string
	kustomize string
	patch     []string
	prune     bool
}

var diffInventoryArgs diffInventoryFlags

func init() {
	diffInventoryCmd.Flags().StringSliceVarP(&diffInventoryArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	diffInventoryCmd.Flags().StringVarP(&diffInventoryArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	diffInventoryCmd.Flags().StringSliceVarP(&diffInventoryArgs.artifact, "artifact", "a", nil,
		"OCI artifact URL in the format 'oci://registry/org/repo:tag' e.g. 'oci://docker.io/stefanprodan/app-deploy:v1.0.0'.")
	diffInventoryCmd.Flags().StringSliceVarP(&diffInventoryArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")
	diffInventoryCmd.Flags().BoolVar(&diffInventoryArgs.prune, "prune", false, "Delete stale objects from the cluster.")
	diffCmd.AddCommand(diffInventoryCmd)
}

func runDiffInventoryCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an inventory name")
	}
	name := args[0]

	if diffInventoryArgs.kustomize == "" && len(diffInventoryArgs.filename) == 0 && len(diffInventoryArgs.artifact) == 0 {
		return fmt.Errorf("-a, -f or -k is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	objects, err := buildManifests(ctx, diffInventoryArgs.kustomize, diffInventoryArgs.filename, diffInventoryArgs.artifact, diffInventoryArgs.patch)
	if err != nil {
		return err
	}

	sort.Sort(ssa.SortableUnstructureds(objects))

	newInventory := inventory.NewInventory(name, *kubeconfigArgs.Namespace)
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

	invStorage := &inventory.Storage{
		Manager: resMgr,
		Owner:   inventoryOwner,
	}

	resMgr.SetOwnerLabels(objects, name, *kubeconfigArgs.Namespace)

	if _, err := exec.LookPath("diff"); err != nil {
		return fmt.Errorf("diff binary not found in PATH, error: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", *kubeconfigArgs.Namespace)
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

	if !invalid {
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
