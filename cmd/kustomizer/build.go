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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/stefanprodan/kustomizer/pkg/objectutil"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build scans the given path for Kubernetes manifests or Kustomize overlays and prints the multi-doc to stdout.",
	RunE:  runBuildCmd,
}

type buildFlags struct {
	filename  []string
	kustomize string
	patch     []string
	output    string
}

var buildArgs buildFlags

func init() {
	buildCmd.Flags().StringSliceVarP(&buildArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	buildCmd.Flags().StringVarP(&buildArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	buildCmd.Flags().StringSliceVarP(&buildArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")
	buildCmd.Flags().StringVarP(&buildArgs.output, "output", "o", "yaml",
		"Write manifests to stdout in YAML or JSON format.")

	rootCmd.AddCommand(buildCmd)
}

func runBuildCmd(cmd *cobra.Command, args []string) error {
	if buildArgs.kustomize == "" && len(buildArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	objects, err := buildManifests(buildArgs.kustomize, buildArgs.filename, buildArgs.patch)
	if err != nil {
		return err
	}

	switch buildArgs.output {
	case "yaml":
		yml, err := objectutil.ObjectsToYAML(objects)
		if err != nil {
			return err
		}
		fmt.Println(yml)
	case "json":
		json, err := objectutil.ObjectsToJSON(objects)
		if err != nil {
			return err
		}
		fmt.Println(json)
	default:
		return fmt.Errorf("unsupported output, can be yaml or json")
	}

	return nil
}

func buildManifests(kustomizePath string, filePaths []string, patchPaths []string) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	if kustomizePath != "" {
		data, err := buildKustomization(kustomizePath)
		if err != nil {
			return nil, err
		}

		objs, err := objectutil.ReadObjects(bytes.NewReader(data))
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

			objs, err := objectutil.ReadObjects(bufio.NewReader(ms))
			ms.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", manifest, err)
			}

			for _, obj := range objs {
				if objectutil.IsKubernetesObject(obj) && !objectutil.IsKustomization(obj) {
					objects = append(objects, obj)
				}
			}
		}
	}

	if len(patchPaths) > 0 {
		for _, patchPath := range patchPaths {
			data, err := applyPatches(patchPath, objects)
			if err != nil {
				return nil, err
			}

			objs, err := objectutil.ReadObjects(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", kustomizePath, err)
			}
			objects = objs
		}
	}

	sort.Sort(objectutil.SortableUnstructureds(objects))
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
	yml, err := objectutil.ObjectsToYAML(objects)
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
