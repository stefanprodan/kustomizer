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

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply Kubernetes manifests and Kustomize overlays using server-side apply.",
	RunE:  runApplyCmd,
}

type applyFlags struct {
	filename  []string
	wait      bool
	force     bool
	kustomize string
	output    string
}

var applyArgs applyFlags

func init() {
	applyCmd.Flags().StringSliceVarP(&applyArgs.filename, "filename", "f", nil, "path to Kubernetes manifest(s)")
	applyCmd.Flags().StringVarP(&applyArgs.kustomize, "kustomize", "k", "", "process a kustomization directory (can't be used together with -f)")
	applyCmd.Flags().BoolVar(&applyArgs.wait, "wait", false, "wait for the applied Kubernetes objects to become ready")
	applyCmd.Flags().BoolVar(&applyArgs.force, "force", false, "recreate objects that contain immutable fields changes")
	applyCmd.Flags().StringVarP(&applyArgs.output, "output", "o", "", "output can be yaml or json")

	rootCmd.AddCommand(applyCmd)
}

func runApplyCmd(cmd *cobra.Command, args []string) error {
	resMgr, err := resmgr.NewResourceManager(rootArgs.kubeconfig, rootArgs.kubecontext, "flagger-cli")
	if err != nil {
		return err
	}

	objects := make([]*unstructured.Unstructured, 0)

	if applyArgs.kustomize != "" {
		data, err := buildKustomization(applyArgs.kustomize)
		if err != nil {
			return err
		}

		objs, err := resMgr.ReadAll(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", applyArgs.kustomize, err)
		}
		objects = append(objects, objs...)
	} else {
		manifests, err := scan(applyArgs.filename)
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

	if applyArgs.output != "" {
		switch applyArgs.output {
		case "yaml":
			yml, err := resMgr.ToYAML(objects)
			if err != nil {
				return err
			}
			fmt.Println(yml)
			return nil
		case "json":
			json, err := resMgr.ToJSON(objects)
			if err != nil {
				return err
			}
			fmt.Println(json)
			return nil
		default:
			return fmt.Errorf("unsupported output, can be yaml or json")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	for _, object := range objects {
		change, err := resMgr.Reconcile(ctx, object, applyArgs.force)
		if err != nil {
			return err
		}
		fmt.Println(change.String())
	}

	if applyArgs.wait {
		fmt.Println("waiting for resources to become ready...")
		err = resMgr.Wait(objects, 2*time.Second, rootArgs.timeout)
		if err != nil {
			return err
		}
		fmt.Println("all resources are ready")
	}

	return nil
}
