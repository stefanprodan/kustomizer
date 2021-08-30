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
	"flag"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
)

var VERSION = "1.0.0-dev.0"

var rootCmd = &cobra.Command{
	Use:           "kustomizer",
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

var rootArgs = rootFlags{}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootArgs.kubeconfig, "kubeconfig", "", "",
		"absolute path to the kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&rootArgs.kubecontext, "context", "", "", "kubernetes context to use")
	rootCmd.PersistentFlags().DurationVar(&rootArgs.timeout, "timeout", time.Minute, "timeout for this operation")

	rootCmd.DisableAutoGenTag = true
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	configureKubeconfig()
	if err := rootCmd.Execute(); err != nil {
		klog.Errorf("%v", err)
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
