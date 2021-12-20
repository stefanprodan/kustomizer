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

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
)

var inspectInventoryCmd = &cobra.Command{
	Use:     "inventory",
	Aliases: []string{"inv"},
	Short:   "Inspect prints the content of the given inventory.",
	Example: ` kustomizer inspect inventory <name> -n <namespace>

  # Get an inventory and list its content
  kustomizer inspect inv my-app -n apps
`,
	RunE: runInspectInventoryCmd,
}

func init() {
	inspectCmd.AddCommand(inspectInventoryCmd)
}

func runInspectInventoryCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an inventory name")
	}
	name := args[0]

	i := inventory.NewInventory(name, *kubeconfigArgs.Namespace)

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

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	if err := invStorage.GetInventory(ctx, i); err != nil {
		return err
	}

	rootCmd.Println(fmt.Sprintf("Inventory: %s/%s", i.Namespace, i.Name))
	rootCmd.Println(fmt.Sprintf("Source: %s", i.Source))
	rootCmd.Println(fmt.Sprintf("Revision: %s", i.Revision))
	rootCmd.Println("Entries:")
	entries, err := i.ListMeta()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		rootCmd.Println("-", ssa.FmtObjMetadata(entry))
	}

	return nil
}
