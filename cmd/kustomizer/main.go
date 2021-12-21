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
	"fmt"
	"os"
	"time"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/stefanprodan/kustomizer/pkg/config"
)

var VERSION = "1.0.0-dev.0"

const PROJECT = "kustomizer"

var rootCmd = &cobra.Command{
	Use:           PROJECT,
	Version:       VERSION,
	SilenceUsage:  true,
	SilenceErrors: true,
	Short:         "A command line utility to publish, fetch, customize, validate, and apply Kubernetes configuration.",
	Long: `Kustomizer is an OSS tool for building Kubernetes continuous delivery workflows.

Distribute Kubernetes configuration as OCI artifacts to container registries:

- kustomizer push artifact oci://<image-url>:<tag> -k [-f] [-p]
- kustomizer tag artifact oci://<image-url>:<tag> <new-tag>
- kustomizer pull artifact oci://<image-url>:<tag>
- kustomizer inspect artifact oci://<image-url>:<tag>

Build, customize and apply Kubernetes resources:

- kustomizer build inventory <name> [-a <oci url>] [-f <dir path>] [-p <patch path>] -k <overlay path>
- kustomizer apply inventory <name> -n <namespace> [-a] [-f] [-p] -k --prune --wait --force
- kustomizer diff inventory <name> -n <namespace> [-a] [-f] [-p] -k

Manage the applied Kubernetes resources:

- kustomizer get inventories --namespace <namespace>
- kustomizer inspect inventory <name> --namespace <namespace>
- kustomizer delete inventory <name> --namespace <namespace>
`,
}

type rootFlags struct {
	timeout time.Duration
}

var (
	rootArgs       = rootFlags{}
	logger         = stderrLogger{stderr: os.Stderr}
	cfg            = config.NewConfig()
	inventoryOwner = ssa.Owner{
		Field: cfg.FieldManager.Name,
		Group: cfg.FieldManager.Group,
	}
)

var kubeconfigArgs = genericclioptions.NewConfigFlags(false)

func init() {
	rootCmd.PersistentFlags().DurationVar(&rootArgs.timeout, "timeout", time.Minute,
		"The length of time to wait before giving up on the current operation.")

	kubeconfigArgs.Timeout = nil
	kubeconfigArgs.Namespace = nil
	kubeconfigArgs.AddFlags(rootCmd.PersistentFlags())

	defaultNamespace := "default"
	kubeconfigArgs.Namespace = &defaultNamespace
	rootCmd.PersistentFlags().StringVarP(kubeconfigArgs.Namespace, "namespace", "n", *kubeconfigArgs.Namespace, "The inventory namespace.")

	rootCmd.DisableAutoGenTag = true
	rootCmd.SetOut(os.Stdout)
}

func main() {
	loadConfig()
	if err := rootCmd.Execute(); err != nil {
		logger.Println(`✗`, err)
		os.Exit(1)
	}
}

func loadConfig() {
	if c, err := config.Read(""); err != nil {
		logger.Println(`✗`, fmt.Errorf("loading the config failed, error: %w", err))
	} else {
		cfg = c
	}

	ssa.ReconcileOrder = ssa.KindOrder{
		First: cfg.ApplyOrder.First,
		Last:  cfg.ApplyOrder.Last,
	}
	inventoryOwner = ssa.Owner{
		Field: cfg.FieldManager.Name,
		Group: cfg.FieldManager.Group,
	}
}
