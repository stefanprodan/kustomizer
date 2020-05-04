package engine

import (
	"fmt"
	"os/exec"
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

	opt := krusty.MakeDefaultOptions()
	opt.DoPrune = true
	k := krusty.MakeKustomizer(b.fs, opt)
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

func (b *Builder) Build(base string, filePath string) error {
	if _, err := exec.LookPath("kustomize"); err != nil {
		return fmt.Errorf("kustomize not found")
	}

	command := fmt.Sprintf("kustomize build %s > %s", base, filePath)
	c := exec.Command("/bin/sh", "-c", command)

	return c.Run()
}
