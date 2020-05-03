package engine

import (
	"fmt"
	"path/filepath"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

type Builder struct {
	fs filesys.FileSystem
}

func NewBuilder(fs filesys.FileSystem) (*Builder, error) {
	return &Builder{fs: fs}, nil
}

// Generate Kubernetes manifests from a kustomization
func (b *Builder) Generate(base string, filePath string) error {
	kfile := filepath.Join(base, "kustomization.yaml")

	if !b.fs.Exists(kfile) {
		return fmt.Errorf("%s not found", kfile)
	}

	k := krusty.MakeKustomizer(b.fs, krusty.MakeDefaultOptions())
	m, err := k.Run(base)
	if err != nil {
		return err
	}

	resources, err := m.AsYaml()
	if err != nil {
		return err
	}

	if err := b.fs.WriteFile(filePath, resources); err != nil {
		return err
	}

	return nil
}
