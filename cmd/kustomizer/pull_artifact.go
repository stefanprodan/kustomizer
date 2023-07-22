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
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/v2/pkg/registry"
)

var pullArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Pull downloads Kubernetes manifests from a container registry.",
	Long: `The pull command downloads the specified OCI artifact and writes the Kubernetes manifests to stdout.
For private registries, the pull command uses the credentials from '~/.docker/config.json'.`,
	Example: `  kustomizer pull artifact <oci url>

  # Pull Kubernetes manifests from an OCI artifact hosted on Docker Hub
  kustomizer pull artifact oci://docker.io/user/repo:v1.0.0 > manifests.yaml

  # Pull an OCI artifact using the digest and write the Kubernetes manifests to stdout
  kustomizer pull artifact oci://docker.io/user/repo@sha256:<digest>

  # Pull the latest artifact from a local registry
  kustomizer pull artifact oci://localhost:5000/repo

  # Pull and verify artifact with cosign
  kustomizer pull artifact oci://docker.io/user/repo:v1.0.0 --verify --cosign-key ./keys/cosign.pub

  # Pull encrypted artifact
  kustomizer pull artifact oci://docker.io/user/repo:v1.0.0 --age-identities ./keys/id.txt
`,
	RunE: runPullArtifactCmd,
}

type pullArtifactFlags struct {
	ageIdentities string
	verify        bool
	verifyKey     string
}

var pullArtifactArgs pullArtifactFlags

func init() {
	pullArtifactCmd.Flags().StringVar(&pullArtifactArgs.ageIdentities, "age-identities", "",
		"Path to a file containing one or more age identities (private keys generated by age-keygen).")
	pullArtifactCmd.Flags().BoolVar(&pullArtifactArgs.verify, "verify", false,
		"Verify the artifact signature with cosign.")
	pullArtifactCmd.Flags().StringVar(&pullArtifactArgs.verifyKey, "cosign-key", "",
		"Path to the consign public key file, KMS URI or Kubernetes Secret. "+
			"When not specified, cosign will try to verify the signature using Rekor.")

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

	if pullArtifactArgs.verify {
		if err := verifyCosign(url, pullArtifactArgs.verifyKey); err != nil {
			return err
		}
	}

	identities, err := registry.ParseAgeIdentities(pullArtifactArgs.ageIdentities)
	if err != nil {
		return fmt.Errorf("faild to read decryption keys: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	yml, _, err := registry.Pull(ctx, url, identities)
	if err != nil {
		return fmt.Errorf("pulling %s failed: %w", url, err)
	}

	rootCmd.Println(yml)
	return nil
}

func verifyCosign(url, key string) error {
	cosign, err := exec.LookPath("cosign")
	if err != nil {
		return fmt.Errorf("cosign not found in path $PATH: %w", err)
	}

	cosignCmd := exec.Command(cosign, []string{"verify"}...)
	cosignCmd.Env = os.Environ()

	if key != "" {
		cosignCmd.Args = append(cosignCmd.Args, "--key", key)
	} else {
		cosignCmd.Env = append(cosignCmd.Env, "COSIGN_EXPERIMENTAL=true")
	}
	cosignCmd.Args = append(cosignCmd.Args, url)

	if msg, err := cosignCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cosign verify failed, %s %w", msg, err)
	}

	return nil
}
