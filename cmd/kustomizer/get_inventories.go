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
	"github.com/stefanprodan/kustomizer/pkg/inventory"
	"github.com/stefanprodan/kustomizer/pkg/manager"
	"github.com/stefanprodan/kustomizer/pkg/objectutil"
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

	if getArgs.namespace == "" {
		return fmt.Errorf("you must specify an intentory namespace")
	}

	kubeClient, err := newKubeClient(rootArgs.kubeconfig, rootArgs.kubecontext)
	if err != nil {
		return fmt.Errorf("client init failed: %w", err)
	}

	statusPoller, err := newKubeStatusPoller(rootArgs.kubeconfig, rootArgs.kubecontext)
	if err != nil {
		return fmt.Errorf("status poller init failed: %w", err)
	}

	resMgr := manager.NewResourceManager(kubeClient, statusPoller, inventoryOwner)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	list := &corev1.ConfigMapList{}
	err = resMgr.Client().List(ctx, list, client.InNamespace(getArgs.namespace), client.MatchingLabels{
		"app.kubernetes.io/component":  "inventory",
		"app.kubernetes.io/created-by": "kustomizer",
	})
	if err != nil {
		return err
	}

	for _, cm := range list.Items {
		fmt.Println(fmt.Sprintf("Inventory: %s/%s", cm.GetNamespace(), cm.GetName()))
		if s, ok := cm.GetAnnotations()["inventory.kustomizer.dev/last-applied-time"]; ok {
			fmt.Println(fmt.Sprintf("LastAppliedTime: %s", s))
		}
		if s, ok := cm.GetAnnotations()["inventory.kustomizer.dev/source"]; ok {
			fmt.Println(fmt.Sprintf("Source: %s", s))
		}
		if s, ok := cm.GetAnnotations()["inventory.kustomizer.dev/revision"]; ok {
			fmt.Println(fmt.Sprintf("Revision: %s", s))
		}
		i := inventory.NewInventory(cm.GetName(), cm.GetNamespace())
		if err := resMgr.GetInventory(ctx, i); err != nil {
			return err
		}

		fmt.Println("Entries:")
		entries, err := i.ListMeta()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			fmt.Println("-", objectutil.FmtObjMetadata(entry))
		}
	}

	return nil
}
