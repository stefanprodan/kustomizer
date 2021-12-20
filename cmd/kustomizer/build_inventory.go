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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
	"sort"
	"strings"
	"sync"

	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var buildInventoryCmd = &cobra.Command{
	Use:     "inventory",
	Aliases: []string{"inv"},
	Short:   "Build generates an inventory and writes the reusing Kubernetes manifests to stdout.",
	Example: `  kustomizer build inventory <name> [-a] [-p] [-f] -k

  # Build the inventory from remote OCI artifacts and print the resulting multi-doc YAML
  kustomizer build inventory my-app -n apps -a oci://registry/org/repo:latest

  # Build the inventory from remote OCI artifacts, apply local patches and print the resulting multi-doc YAML
  kustomizer build inventory my-app -n apps -a oci://registry/org/repo:latest -p ./patches/safe-to-evict.yaml

  # Build the inventory from local files and print the resulting multi-doc YAML
  kustomizer build inventory my-app -n apps -f ./deploy/manifests

  # Build the inventory from a local overlay and print the resulting multi-doc YAML
  kustomizer build inventory my-app -n apps -k ./overlays/prod
`,
	RunE: runBuildInventoryCmd,
}

type buildInventoryFlags struct {
	artifact  []string
	filename  []string
	kustomize string
	patch     []string
	output    string
}

var buildInventoryArgs buildInventoryFlags

func init() {
	buildInventoryCmd.Flags().StringSliceVarP(&buildInventoryArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	buildInventoryCmd.Flags().StringVarP(&buildInventoryArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	buildInventoryCmd.Flags().StringSliceVarP(&buildInventoryArgs.artifact, "artifact", "a", nil,
		"OCI artifact URL in the format 'oci://registry/org/repo:tag' e.g. 'oci://docker.io/stefanprodan/app-deploy:v1.0.0'.")
	buildInventoryCmd.Flags().StringSliceVarP(&buildInventoryArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")
	buildInventoryCmd.Flags().StringVarP(&buildInventoryArgs.output, "output", "o", "yaml",
		"Write manifests to stdout in YAML or JSON format.")

	buildCmd.AddCommand(buildInventoryCmd)
}

func runBuildInventoryCmd(cmd *cobra.Command, args []string) error {
	if buildInventoryArgs.kustomize == "" && len(buildInventoryArgs.filename) == 0 && len(buildInventoryArgs.artifact) == 0 {
		return fmt.Errorf("-a, -f or -k is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	objects, err := buildManifests(ctx, buildInventoryArgs.kustomize, buildInventoryArgs.filename, buildInventoryArgs.artifact, buildInventoryArgs.patch)
	if err != nil {
		return err
	}

	sort.Sort(ssa.SortableUnstructureds(objects))

	switch buildInventoryArgs.output {
	case "yaml":
		yml, err := ssa.ObjectsToYAML(objects)
		if err != nil {
			return err
		}
		rootCmd.Println(yml)
	case "json":
		json, err := ssa.ObjectsToJSON(objects)
		if err != nil {
			return err
		}
		rootCmd.Println(json)
	default:
		return fmt.Errorf("unsupported output, can be yaml or json")
	}

	return nil
}

func buildManifests(ctx context.Context, kustomizePath string, filePaths []string, artifacts []string, patchPaths []string) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	if kustomizePath != "" {
		data, err := buildKustomization(kustomizePath)
		if err != nil {
			return nil, err
		}

		objs, err := ssa.ReadObjects(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", kustomizePath, err)
		}
		objects = append(objects, objs...)
	}

	if len(filePaths) > 0 {
		manifests, err := scanForManifests(filePaths)
		if err != nil {
			return nil, err
		}
		for _, manifest := range manifests {
			ms, err := os.Open(manifest)
			if err != nil {
				return nil, err
			}

			objs, err := ssa.ReadObjects(bufio.NewReader(ms))
			ms.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", manifest, err)
			}

			for _, obj := range objs {
				if ssa.IsKubernetesObject(obj) && !ssa.IsKustomization(obj) {
					objects = append(objects, obj)
				}
			}
		}
	}

	if len(artifacts) > 0 {
		for _, ociURL := range artifacts {
			url, err := registry.ParseURL(ociURL)
			if err != nil {
				return nil, fmt.Errorf("parsing %s failed: %w", ociURL, err)
			}

			yml, _, err := registry.Pull(ctx, url)
			if err != nil {
				return nil, fmt.Errorf("pulling %s failed: %w", ociURL, err)
			}

			objs, err := ssa.ReadObjects(strings.NewReader(yml))
			if err != nil {
				return nil, fmt.Errorf("extracting manifests from %s failed: %w", ociURL, err)
			}
			objects = append(objects, objs...)
		}
	}

	if len(patchPaths) > 0 {
		for _, patchPath := range patchPaths {
			data, err := applyPatches(patchPath, objects)
			if err != nil {
				return nil, err
			}

			objs, err := ssa.ReadObjects(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", kustomizePath, err)
			}
			objects = objs
		}
	}

	return objects, nil
}

func scanForManifests(paths []string) ([]string, error) {
	var manifests []string

	for _, in := range paths {
		fi, err := os.Stat(in)
		if err != nil {
			return nil, err
		}

		switch mode := fi.Mode(); {
		case mode.IsDir():
			m, err := scanRec(in)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, m...)
		case mode.IsRegular():
			if matchExt(fi.Name()) {
				manifests = append(manifests, in)
			}
		}
	}

	return manifests, nil
}

func scanRec(dir string) ([]string, error) {
	var manifests []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			m, err := scanRec(path.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, m...)
		}
		if matchExt(file.Name()) {
			manifests = append(manifests, path.Join(dir, file.Name()))
		}
	}
	return manifests, err
}

func matchExt(f string) bool {
	ext := path.Ext(f)
	return ext == ".yaml" || ext == ".yml"
}

var kustomizeBuildMutex sync.Mutex

func buildKustomization(base string) ([]byte, error) {
	kustomizeBuildMutex.Lock()
	defer kustomizeBuildMutex.Unlock()

	kfile := path.Join(base, "kustomization.yaml")

	fs := filesys.MakeFsOnDisk()
	if !fs.Exists(kfile) {
		return nil, fmt.Errorf("%s not found", kfile)
	}

	if path.IsAbs(base) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		base, err = filepath.Rel(wd, base)
		if err != nil {
			return nil, err
		}
	}

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	m, err := k.Run(fs, base)
	if err != nil {
		return nil, err
	}

	resources, err := m.AsYaml()
	if err != nil {
		return nil, err
	}

	return resources, nil
}

func applyPatches(kFilePath string, objects []*unstructured.Unstructured) ([]byte, error) {
	kustomizeBuildMutex.Lock()
	defer kustomizeBuildMutex.Unlock()

	data, err := ioutil.ReadFile(kFilePath)
	if err != nil {
		return nil, err
	}

	template := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, err
	}

	if len(template.Patches) == 0 {
		return nil, fmt.Errorf("no patches found in %s", kFilePath)
	}

	fs := filesys.MakeFsInMemory()
	kustomization := kustypes.Kustomization{}
	kustomization.APIVersion = kustypes.KustomizationVersion
	kustomization.Kind = kustypes.KustomizationKind

	const input = "resources.yaml"
	kustomization.Resources = append(kustomization.Resources, input)
	yml, err := ssa.ObjectsToYAML(objects)
	if err != nil {
		return nil, err
	}

	if err := fs.WriteFile(input, []byte(yml)); err != nil {
		return nil, err
	}

	kustomization.Patches = template.Patches

	d, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, err
	}

	if err := fs.WriteFile("kustomization.yaml", d); err != nil {
		return nil, err
	}

	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	m, err := k.Run(fs, ".")
	if err != nil {
		return nil, err
	}

	resources, err := m.AsYaml()
	if err != nil {
		return nil, err
	}

	return resources, nil
}
