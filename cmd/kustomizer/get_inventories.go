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
	"io"

	"github.com/fluxcd/pkg/ssa"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
)

var getInventories = &cobra.Command{
	Use:     "inventory",
	Aliases: []string{"inv", "inventories"},
	Short:   "Get prints a table with the inventories in the given namespace.",
	Example: ` kustomizer get inventories -n <namespace>

  # Get an inventory in the specified namespace
  kustomizer get inventory my-app -n apps

  # Get all inventories in the specified namespace
  kustomizer get inventories -n apps
`,
	RunE: runGetInventoriesCmd,
}

type getInventoriesFlags struct {
	allNamespaces bool
}

var getInventoriesArgs getInventoriesFlags

func init() {
	getInventories.Flags().BoolVar(&getInventoriesArgs.allNamespaces, "all-namespaces", false,
		"list the requested object(s) across all namespaces.")

	getCmd.AddCommand(getInventories)
}

func runGetInventoriesCmd(cmd *cobra.Command, args []string) error {

	if kubeconfigArgs.Namespace == nil {
		return fmt.Errorf("you must specify an inventory namespace")
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

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	ns := *kubeconfigArgs.Namespace
	if getInventoriesArgs.allNamespaces {
		ns = ""
	}
	inventories, err := invStorage.ListInventories(ctx, ns)
	if err != nil {
		return err
	}

	var rows [][]string
	for _, inv := range inventories {
		row := []string{}
		if getInventoriesArgs.allNamespaces {
			row = []string{inv.Name, inv.Namespace, fmt.Sprintf("%v", len(inv.Entries)), inv.Source, inv.Revision, inv.LastAppliedTime}
		} else {
			row = []string{inv.Name, fmt.Sprintf("%v", len(inv.Entries)), inv.Source, inv.Revision, inv.LastAppliedTime}
		}
		rows = append(rows, row)
	}

	if getInventoriesArgs.allNamespaces {
		printTable(rootCmd.OutOrStdout(), []string{"name", "namespace", "entries", "source", "revision", "last applied"}, rows)
	} else {
		printTable(rootCmd.OutOrStdout(), []string{"name", "entries", "source", "revision", "last applied"}, rows)
	}

	return nil
}

func printTable(writer io.Writer, header []string, rows [][]string) {
	table := tablewriter.NewWriter(writer)
	table.SetHeader(header)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(rows)
	table.Render()
}
