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

	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var pullArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Pull downloads Kubernetes manifests from a container registry.",
	Long: `The pull command downloads the specified OCI artifact and writes the Kubernetes manifests to stdout.
For private registries, the pull command uses the credentials from '~/.docker/config.json'.`,
	Example: `  kustomizer pull artifact <oci url>

  # Pull Kubernetes manifests from an OCI artifact hosted on Docker Hub
  kustomizer pull oci://docker.io/user/repo:v1.0.0 > manifests.yaml

  # Pull an OCI artifact using the digest and write the Kubernetes manifests to stdout
  kustomizer pull oci://docker.io/user/repo@sha256:<digest>

  # Pull the latest artifact from a local registry
  kustomizer pull oci://localhost:5000/repo

  # Apply Kubernetes manifests from an OCI artifact 
  kustomizer pull oci://docker.io/user/repo:v1.0.0 | kustomizer apply -i test -f-
`,
	RunE: runPullArtifactCmd,
}

func init() {
	pullCmd.AddCommand(pullArtifactCmd)
}

func runPullArtifactCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an artifact name e.g. 'oci://docker.io/user/repo:tag'")
	}

	url, err := registry.ParseURL(args[0])
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	yml, _, err := registry.Pull(ctx, url)
	if err != nil {
		return fmt.Errorf("pulling %s failed: %w", url, err)
	}

	rootCmd.Println(yml)
	return nil
}
