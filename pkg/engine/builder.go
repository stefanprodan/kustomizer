package engine

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resource"
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

	// check if resources are SOPS encrypted and decrypt them before
	// generating the final YAML
	for _, res := range m.Resources() {
		outRes, err := b.decryptSOPS(res)
		if err != nil {
			return err
		}

		if outRes != nil {
			_, err = m.Replace(res)
			if err != nil {
				return err
			}
		}
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

func (b *Builder) decryptSOPS(res *resource.Resource) (*resource.Resource, error) {
	out, err := res.AsYAML()
	if err != nil {
		return nil, err
	}

	if bytes.Contains(out, []byte("sops:")) && bytes.Contains(out, []byte("mac: ENC[")) {
		cmd := fmt.Sprintf("echo \"%s\" | sops --input-type yaml --output-type json -d --ignore-mac /dev/stdin", string(out))
		command := exec.Command("/bin/sh", "-c", cmd)
		outDec, err := command.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("sops error: %s %w", strings.TrimSuffix(string(outDec), "\n"), err)
		}

		err = res.UnmarshalJSON(outDec)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	return nil, nil
}

func (b *Builder) Build(base string, filePath string) error {
	if _, err := exec.LookPath("kustomize"); err != nil {
		return fmt.Errorf("kustomize not found")
	}

	command := fmt.Sprintf("kustomize build %s > %s", base, filePath)
	c := exec.Command("/bin/sh", "-c", command)
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", command)
	}
	return c.Run()
}
