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
	"github.com/stefanprodan/kustomizer/pkg/resmgr"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/inventory"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var VERSION = "1.0.0-dev.0"

const PROJECT = "kustomizer"

var rootCmd = &cobra.Command{
	Use:           PROJECT,
	Version:       VERSION,
	SilenceUsage:  true,
	SilenceErrors: true,
	Short:         "A command line utility for reconciling Kubernetes manifests and Kustomize overlays.",
}

type rootFlags struct {
	kubeconfig  string
	kubecontext string
	timeout     time.Duration
}

var (
	rootArgs          = rootFlags{}
	logger            = stderrLogger{stderr: os.Stderr}
	resourceFormatter = &resmgr.ResourceFormatter{}
	inventoryMgr      *inventory.InventoryManager
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootArgs.kubeconfig, "kubeconfig", "", "",
		"Absolute path to the kubeconfig file.")
	rootCmd.PersistentFlags().StringVarP(&rootArgs.kubecontext, "context", "", "",
		"The Kubernetes context to use.")
	rootCmd.PersistentFlags().DurationVar(&rootArgs.timeout, "timeout", time.Minute,
		"The length of time to wait before giving up on the current operation.")

	rootCmd.DisableAutoGenTag = true
}

func main() {
	configureKubeconfig()

	if im, err := inventory.NewInventoryManager(PROJECT, PROJECT+".dev"); err != nil {
		panic(err)
	} else {
		inventoryMgr = im
	}

	if err := rootCmd.Execute(); err != nil {
		logger.Println(err)
		os.Exit(1)
	}
}

func configureKubeconfig() {
	switch {
	case len(rootArgs.kubeconfig) > 0:
	case len(os.Getenv("KUBECONFIG")) > 0:
		rootArgs.kubeconfig = os.Getenv("KUBECONFIG")
	default:
		if home := homeDir(); len(home) > 0 {
			rootArgs.kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
