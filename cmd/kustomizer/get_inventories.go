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
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/inventory"
	"io"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var getInventories = &cobra.Command{
	Use:   "inventories",
	Short: "Get prints the content of all inventories in the given namespace.",
	RunE:  runGetInventoriesCmd,
}

func init() {
	getCmd.AddCommand(getInventories)
}

func runGetInventoriesCmd(cmd *cobra.Command, args []string) error {

	if kubeconfigArgs.Namespace == nil {
		return fmt.Errorf("you must specify an intentory namespace")
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

	list := &corev1.ConfigMapList{}
	err = resMgr.Client().List(ctx, list, client.InNamespace(*kubeconfigArgs.Namespace), client.MatchingLabels{
		"app.kubernetes.io/component":  "inventory",
		"app.kubernetes.io/created-by": "kustomizer",
	})
	if err != nil {
		return err
	}

	var rows [][]string
	for _, cm := range list.Items {
		var ts string
		var source string
		var rev string
		if s, ok := cm.GetAnnotations()["inventory.kustomizer.dev/last-applied-time"]; ok {
			ts = s
		}
		if s, ok := cm.GetAnnotations()["inventory.kustomizer.dev/source"]; ok {
			source = s
		}
		if s, ok := cm.GetAnnotations()["inventory.kustomizer.dev/revision"]; ok {
			rev = s
		}
		i := inventory.NewInventory(cm.GetName(), cm.GetNamespace())
		if err := invStorage.GetInventory(ctx, i); err != nil {
			return err
		}
		row := []string{cm.GetName(), fmt.Sprintf("%v", len(i.Entries)), source, rev, ts}
		rows = append(rows, row)
	}

	printTable(rootCmd.OutOrStdout(), []string{"name", "entries", "source", "revision", "last applied"}, rows)

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
