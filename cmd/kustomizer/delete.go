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
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/resmgr"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the Kubernetes objects in the inventory",
	RunE:  deleteCmdRun,
}

type deleteFlags struct {
	inventoryName      string
	inventoryNamespace string
	wait               bool
}

var deleteArgs deleteFlags

func init() {
	deleteCmd.Flags().StringVarP(&deleteArgs.inventoryName, "inventory-name", "i", "", "inventory configmap name")
	deleteCmd.Flags().StringVar(&deleteArgs.inventoryNamespace, "inventory-namespace", "default", "inventory configmap namespace")
	deleteCmd.Flags().BoolVar(&deleteArgs.wait, "wait", true, "wait for the deleted Kubernetes objects to be terminated")

	rootCmd.AddCommand(deleteCmd)
}

func deleteCmdRun(cmd *cobra.Command, args []string) error {
	if deleteArgs.inventoryName == "" {
		return fmt.Errorf("--inventory-name is required")
	}
	if deleteArgs.inventoryNamespace == "" {
		return fmt.Errorf("--inventory-namespace is required")
	}

	resMgr, err := resmgr.NewResourceManager(rootArgs.kubeconfig, rootArgs.kubecontext, PROJECT)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	inv, err := inventoryMgr.Retrieve(ctx, resMgr.KubeClient(), deleteArgs.inventoryName, deleteArgs.inventoryNamespace)
	objects, err := inv.List()
	if err != nil {
		return err
	}

	changeSet, err := resMgr.DeleteAll(ctx, objects)
	if err != nil {
		return err
	}
	for _, change := range changeSet.Entries {
		fmt.Println(change.String())
	}

	err = inventoryMgr.Remove(ctx, resMgr.KubeClient(), deleteArgs.inventoryName, deleteArgs.inventoryNamespace)
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("ConfigMap/%s/%s deleted", deleteArgs.inventoryNamespace, deleteArgs.inventoryName))

	if deleteArgs.wait {
		fmt.Println("waiting for resources to be terminated...")
		err = resMgr.WaitForTermination(objects, 2*time.Second, rootArgs.timeout)
		if err != nil {
			return err
		}
		fmt.Println("all resources have been deleted")
	}

	return nil
}
