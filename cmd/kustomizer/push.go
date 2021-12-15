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
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var pushCmd = &cobra.Command{
	Use:   "push OCIURL",
	Short: "Push uploads Kubernetes manifests to a container registry.",
	Long: `The push command scans the given path for Kubernetes manifests or Kustomize overlays,
builds the manifests into a multi-doc YAML, packages the YAML file into an OCI artifact and
pushes the image to the container registry.
The push command uses the credentials from '~/.docker/config.json'.`,
	Example: `  # Build Kubernetes plain manifests and push the resulting multi-doc YAML to Docker Hub
  kustomizer push oci://docker.io/user/repo:v1.0.0 -f ./deploy/manifests

  # Build a Kustomize overlay and push the resulting multi-doc YAML to GitHub Container Registry
  kustomizer push oci://ghcr.io/user/repo:v1.0.0 -k ./deploy/production 

  # Push to a local registry
  kustomizer push oci://localhost:5000/repo:latest -f ./deploy/manifests 
`,
	RunE: runPushCmd,
}

type pushFlags struct {
	filename  []string
	kustomize string
	patch     []string
}

var pushArgs pushFlags

func init() {
	pushCmd.Flags().StringSliceVarP(&pushArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	pushCmd.Flags().StringVarP(&pushArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	pushCmd.Flags().StringSliceVarP(&pushArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")

	rootCmd.AddCommand(pushCmd)
}

func runPushCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an artifact name e.g. 'oci://docker.io/user/repo:tag'")
	}

	if pushArgs.kustomize == "" && len(pushArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	url, err := registry.ParseURL(args[0])
	if err != nil {
		return err
	}

	logger.Println("building manifests...")
	objects, err := buildManifests(pushArgs.kustomize, pushArgs.filename, pushArgs.patch)
	if err != nil {
		return err
	}

	sort.Sort(ssa.SortableUnstructureds(objects))

	for _, object := range objects {
		rootCmd.Println(ssa.FmtUnstructured(object))
	}

	yml, err := ssa.ObjectsToYAML(objects)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	logger.Println("pushing image", url)
	digest, err := registry.Push(ctx, url, yml, &registry.Metadata{
		Version:  VERSION,
		Checksum: fmt.Sprintf("%x", sha256.Sum256([]byte(yml))),
	})
	if err != nil {
		return fmt.Errorf("pushing image failed: %w", err)
	}

	logger.Println("published digest", digest)

	return nil
}
