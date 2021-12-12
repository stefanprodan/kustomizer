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
	"os"
	"path/filepath"
	"sort"

	"github.com/fluxcd/pkg/ssa"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push Kubernetes manifests to an OCI registry.",
	Long: `The push command scans the given path for Kubernetes manifests or Kustomize overlays,
builds the manifests into a multi-doc YAML, packages the YAML file into an OCI artifact and
pushes the artifact to the specified image repository.
The push command uses the credentials from '~/.docker/config.json'.`,
	Example: `  # Build Kubernetes plain manifests and push the resulting multi-doc YAML to Docker Hub
  kustomizer push -f ./deploy/manifests docker.io/user/repo:v1.0.0

  # Build a Kustomize overlay and push the resulting multi-doc YAML to GitHub Container Registry
  kustomizer push -k ./deploy/production ghcr.io/user/repo:v1.0.0
`,
	RunE: runPushCmd,
}

type pushFlags struct {
	filename  []string
	kustomize string
	patch     []string
	output    string
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
		return fmt.Errorf("you must specify an artifact name e.g. 'docker.io/user/repo:tag'")
	}
	url := args[0]

	if pushArgs.kustomize == "" && len(pushArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	objects, err := buildManifests(pushArgs.kustomize, pushArgs.filename, pushArgs.patch)
	if err != nil {
		return err
	}

	sort.Sort(ssa.SortableUnstructureds(objects))

	yml, err := ssa.ObjectsToYAML(objects)
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(yml)))

	tmpDir, err := os.MkdirTemp("", "kustomizer")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarFile := filepath.Join(tmpDir, "all.tar")
	if err := tarYAML(tarFile, "all.yaml", yml); err != nil {
		return err
	}

	img, err := crane.Append(empty.Image, tarFile)
	if err != nil {
		return err
	}

	annotations := map[string]string{
		"kustomizer.dev/version":  VERSION,
		"kustomizer.dev/checksum": checksum,
	}
	img = mutate.Annotations(img, annotations).(gcrv1.Image)

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

	if err := crane.Push(img, url, options...); err != nil {
		return fmt.Errorf("pushing image %s: %w", url, err)
	}
	ref, err := name.ParseReference(url)
	if err != nil {
		return fmt.Errorf("parsing reference %s: %w", url, err)
	}
	d, err := img.Digest()
	if err != nil {
		return fmt.Errorf("digest: %w", err)
	}
	fmt.Println(ref.Context().Digest(d.String()))

	return nil
}

func tarYAML(tarPath string, name, data string) error {
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()
	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	header := &tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(data)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write([]byte(data)); err != nil {
		return err
	}

	return nil
}
