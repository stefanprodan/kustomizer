package ksync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

type Generator struct {
	rv *Revisor
	fs filesys.FileSystem
}

func NewGenerator(fs filesys.FileSystem, revisor *Revisor) (*Generator, error) {
	if revisor == nil {
		return nil, fmt.Errorf("revisor is nil")
	}

	return &Generator{
		rv: revisor,
		fs: fs,
	}, nil
}

// Generate kustomization file or append label transformer to an existing one
func (g *Generator) Generate(base string) error {
	kfile := filepath.Join(base, "kustomization.yaml")
	if g.fs.Exists(kfile) {
		if err := g.edit(base); err != nil {
			return err
		}
	} else {
		if err := g.create(base); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) create(base string) error {
	kfile := filepath.Join(base, "kustomization.yaml")

	path, err := filepath.Abs(base)
	if err != nil {
		return err
	}

	files, err := g.scan(path, true)
	if err != nil {
		return err
	}

	f, err := g.fs.Create(kfile)
	if err != nil {
		return err
	}
	f.Close()

	kustomization := types.Kustomization{
		TypeMeta: types.TypeMeta{
			APIVersion: types.KustomizationVersion,
			Kind:       types.KustomizationKind,
		},
	}

	var resources []string

	for _, file := range files {
		if _, name := filepath.Split(file); name == g.rv.LabelsFile() {
			continue
		}
		resources = append(resources, strings.Replace(file, path, ".", 1))
	}

	kustomization.Resources = resources
	kustomization.Transformers = []string{g.rv.LabelsFile()}

	data, err := yaml.Marshal(kustomization)
	if err != nil {
		return err
	}

	if err := g.fs.WriteFile(kfile, data); err != nil {
		return err
	}
	return nil
}

func (g *Generator) scan(base string, recursive bool) ([]string, error) {
	var paths []string
	uf := kunstruct.NewKunstructuredFactoryImpl()
	err := g.fs.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == base {
			return nil
		}
		if info.IsDir() {
			if !recursive {
				return filepath.SkipDir
			}
			// If a sub-directory contains an existing kustomization file add the
			// directory as a resource and do not decend into it.
			for _, kfilename := range konfig.RecognizedKustomizationFileNames() {
				if g.fs.Exists(filepath.Join(path, kfilename)) {
					paths = append(paths, path)
					return filepath.SkipDir
				}
			}
			return nil
		}
		fContents, err := g.fs.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := uf.SliceFromBytes(fContents); err != nil {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

func (g *Generator) edit(base string) error {
	kfile := filepath.Join(base, "kustomization.yaml")

	f, err := g.fs.ReadFile(kfile)
	if err != nil {
		return err
	}

	kustomization := types.Kustomization{
		TypeMeta: types.TypeMeta{
			APIVersion: types.KustomizationVersion,
			Kind:       types.KustomizationKind,
		},
	}

	if err := yaml.Unmarshal(f, &kustomization); err != nil {
		return err
	}

	if len(kustomization.Transformers) == 0 {
		kustomization.Transformers = []string{g.rv.LabelsFile()}
	} else {
		var exists bool
		for _, transformer := range kustomization.Transformers {
			if transformer == g.rv.LabelsFile() {
				exists = true
				break
			}
		}
		if !exists {
			kustomization.Transformers = append(kustomization.Transformers, g.rv.LabelsFile())
		}
	}

	data, err := yaml.Marshal(kustomization)
	if err != nil {
		return err
	}

	if err := g.fs.WriteFile(kfile, data); err != nil {
		return err
	}

	return nil
}
