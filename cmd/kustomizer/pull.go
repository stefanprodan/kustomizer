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
	"archive/tar"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull downloads Kubernetes manifests from an OCI registry.",
	Long: `The pull command downloads the specified OCI artifact and writes the Kubernetes manifests to stdout.
For private registries, the pull command uses the credentials from '~/.docker/config.json'.`,
	Example: `  # Pull Kubernetes manifests from an OCI artifact hosted on Docker Hub
  kustomizer pull docker.io/user/repo:v1.0.0
`,
	RunE: runPullCmd,
}

func init() {
	rootCmd.AddCommand(pullCmd)
}

func runPullCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an artifact name e.g. 'docker.io/user/repo:tag'")
	}
	url := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	options := []crane.Option{}
	options = append(options,
		crane.WithContext(ctx),
		crane.WithUserAgent("kustomizer/v1"),
		crane.WithPlatform(&gcrv1.Platform{
			Architecture: "none",
			OS:           "none",
		}),
	)

	img, err := crane.Pull(url, options...)
	if err != nil {
		return err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return err
	}

	if _, ok := manifest.Annotations["kustomizer.dev/version"]; !ok {
		return fmt.Errorf("'kustomizer.dev/version' annotation not found in the OCI manifest")
	}

	checksum, ok := manifest.Annotations["kustomizer.dev/checksum"]
	if !ok {
		return fmt.Errorf("'kustomizer.dev/checksum' annotation not found in the OCI manifest")
	}

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	if len(layers) < 1 {
		return fmt.Errorf("no layers not found")
	}

	blob, err := layers[0].Uncompressed()
	if err != nil {
		return err
	}

	yml, err := untarYAML(blob)
	if err != nil {
		return err
	}

	if checksum != fmt.Sprintf("%x", sha256.Sum256([]byte(yml))) {
		return fmt.Errorf("checksum mismatch")
	}

	rootCmd.Println(yml)
	return nil
}

func untarYAML(r io.Reader) (string, error) {
	sb := new(strings.Builder)
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return sb.String(), nil
		case err != nil:
			return "", err
		case header == nil:
			continue
		}

		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(sb, tr); err != nil {
				return "", err
			}
		}
	}
}
