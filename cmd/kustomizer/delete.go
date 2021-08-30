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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/resmgr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Kubernetes objects previous applied",
	RunE:  deleteCmdRun,
}

type deleteFlags struct {
	filename  []string
	kustomize string
	wait      bool
}

var deleteArgs deleteFlags

func init() {
	deleteCmd.Flags().StringSliceVarP(&deleteArgs.filename, "filename", "f", nil, "path to Kubernetes manifest(s)")
	deleteCmd.Flags().StringVarP(&deleteArgs.kustomize, "kustomize", "k", "", "process a kustomization directory (can't be used together with -f)")
	deleteCmd.Flags().BoolVar(&deleteArgs.wait, "wait", false, "wait for the deleted Kubernetes objects to be terminated")

	rootCmd.AddCommand(deleteCmd)
}

func deleteCmdRun(cmd *cobra.Command, args []string) error {
	resMgr, err := resmgr.NewResourceManager(rootArgs.kubeconfig, rootArgs.kubecontext, "flagger-cli")
	if err != nil {
		return err
	}

	objects := make([]*unstructured.Unstructured, 0)

	if applyArgs.kustomize != "" {
		data, err := buildKustomization(deleteArgs.kustomize)
		if err != nil {
			return err
		}

		objs, err := resMgr.ReadAll(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", deleteArgs.kustomize, err)
		}
		objects = append(objects, objs...)
	} else {
		if len(deleteArgs.filename) == 0 {
			return fmt.Errorf("-f or -k is required")
		}
		manifests, err := scan(deleteArgs.filename)
		if err != nil {
			return err
		}
		for _, manifest := range manifests {
			ms, err := os.Open(manifest)
			if err != nil {
				return err
			}

			objs, err := resMgr.ReadAll(bufio.NewReader(ms))
			ms.Close()
			if err != nil {
				return fmt.Errorf("%s: %w", manifest, err)
			}
			objects = append(objects, objs...)
		}
	}
	sort.Sort(resmgr.ApplyOrder(objects))

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	changeSet, err := resMgr.DeleteAll(ctx, objects)
	if err != nil {
		return err
	}
	for _, change := range changeSet.Entries {
		fmt.Println(change.String())
	}

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
