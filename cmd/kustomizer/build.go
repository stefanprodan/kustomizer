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
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/stefanprodan/kustomizer/pkg/resmgr"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a set of Kubernetes manifests or Kustomize overlays.",
	RunE:  runBuildCmd,
}

type buildFlags struct {
	filename  []string
	kustomize string
	output    string
}

var buildArgs buildFlags

func init() {
	buildCmd.Flags().StringSliceVarP(&buildArgs.filename, "filename", "f", nil, "path to Kubernetes manifest(s)")
	buildCmd.Flags().StringVarP(&buildArgs.kustomize, "kustomize", "k", "", "process a kustomization directory (can't be used together with -f)")
	buildCmd.Flags().StringVarP(&buildArgs.output, "output", "o", "yaml", "output can be yaml or json")

	rootCmd.AddCommand(buildCmd)
}

func runBuildCmd(cmd *cobra.Command, args []string) error {
	if buildArgs.kustomize == "" && len(buildArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	objects, err := buildManifests(buildArgs.kustomize, buildArgs.filename)
	if err != nil {
		return err
	}

	switch buildArgs.output {
	case "yaml":
		yml, err := inventoryMgr.ToYAML(objects)
		if err != nil {
			return err
		}
		fmt.Println(yml)
	case "json":
		json, err := inventoryMgr.ToJSON(objects)
		if err != nil {
			return err
		}
		fmt.Println(json)
	default:
		return fmt.Errorf("unsupported output, can be yaml or json")
	}

	return nil
}

func buildManifests(kustomizePath string, filePaths []string) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	if kustomizePath != "" {
		data, err := buildKustomization(kustomizePath)
		if err != nil {
			return nil, err
		}

		objs, err := inventoryMgr.ReadAll(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", kustomizePath, err)
		}
		objects = append(objects, objs...)
	}

	if len(filePaths) > 0 {
		manifests, err := scan(filePaths)
		if err != nil {
			return nil, err
		}
		for _, manifest := range manifests {
			ms, err := os.Open(manifest)
			if err != nil {
				return nil, err
			}

			objs, err := inventoryMgr.ReadAll(bufio.NewReader(ms))
			ms.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", manifest, err)
			}
			objects = append(objects, objs...)
		}
	}

	sort.Sort(resmgr.ApplyOrder(objects))
	return objects, nil
}

func scan(paths []string) ([]string, error) {
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
