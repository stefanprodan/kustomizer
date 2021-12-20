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
	"sort"

	"github.com/stefanprodan/kustomizer/pkg/inventory"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
)

var deleteInventoryCmd = &cobra.Command{
	Use:     "inventory",
	Aliases: []string{"inv"},
	Short:   "Delete the Kubernetes objects in specified inventory including the inventory storage.",
	Example: ` kustomizer delete inventory <inventory name> -n <inventory namespace>

  # Delete an inventory and its content
  kustomizer delete inv my-app -n apps
`,
	RunE: deleteInventoryCmdRun,
}

type deleteInventoryFlags struct {
	wait bool
}

var deleteInventoryArgs deleteInventoryFlags

func init() {
	deleteInventoryCmd.Flags().BoolVar(&deleteInventoryArgs.wait, "wait", true, "Wait for the deleted Kubernetes objects to be terminated.")

	deleteCmd.AddCommand(deleteInventoryCmd)
}

func deleteInventoryCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an inventory name")
	}
	name := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	logger.Println("retrieving inventory...")

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

	inv := inventory.NewInventory(name, *kubeconfigArgs.Namespace)
	if err := invStorage.GetInventory(ctx, inv); err != nil {
		return err
	}

	objects, err := inv.ListObjects()
	if err != nil {
		return err
	}

	logger.Println(fmt.Sprintf("deleting %v manifest(s)...", len(objects)))
	hasErrors := false
	sort.Sort(sort.Reverse(ssa.SortableUnstructureds(objects)))
	for _, object := range objects {
		change, err := resMgr.Delete(ctx, object, ssa.DefaultDeleteOptions())
		if err != nil {
			logger.Println(`✗`, err)
			hasErrors = true
			continue
		}
		logger.Println(change.String())
	}

	if hasErrors {
		os.Exit(1)
	}

	if err := invStorage.DeleteInventory(ctx, inv); err != nil {
		return err
	}

	logger.Println(fmt.Sprintf("ConfigMap/%s/%s deleted", *kubeconfigArgs.Namespace, name))

	if deleteInventoryArgs.wait {
		waitOpts := ssa.DefaultWaitOptions()
		waitOpts.Timeout = rootArgs.timeout
		logger.Println("waiting for resources to be terminated...")
		err = resMgr.WaitForTermination(objects, waitOpts)
		if err != nil {
			return err
		}
		logger.Println("all resources have been deleted")
	}

	return nil
}
