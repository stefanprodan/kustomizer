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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
)

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
