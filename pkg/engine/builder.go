package engine

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.mozilla.org/sops/v3/aes"
	"go.mozilla.org/sops/v3/cmd/sops/common"
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/yaml"
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
		store := common.StoreForFormat(formats.Yaml)

		// Load SOPS file and access the data key
		tree, err := store.LoadEncryptedFile(out)
		if err != nil {
			return nil, fmt.Errorf("LoadEncryptedFile: %w", err)
		}
		key, err := tree.Metadata.GetDataKey()
		if err != nil {
			return nil, fmt.Errorf("GetDataKey: %w", err)
		}

		// Decrypt the tree
		cipher := aes.NewCipher()
		if _, err := tree.Decrypt(key, cipher); err != nil {
			return nil, fmt.Errorf("Decrypt: %w", err)
		}

		data, err := store.EmitPlainFile(tree.Branches)
		if err != nil {
			return nil, fmt.Errorf("EmitPlainFile: %w", err)
		}

		jsonData, err := yaml.YAMLToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("YAMLToJSON: %w", err)
		}

		err = res.UnmarshalJSON(jsonData)
		if err != nil {
			return nil, fmt.Errorf("UnmarshalJSON: %w", err)
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
