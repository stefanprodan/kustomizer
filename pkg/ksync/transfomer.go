package ksync

import (
	"fmt"
	"path/filepath"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

type Transformer struct {
	rv *Revisor
	fs filesys.FileSystem
}

func NewTransformer(fs filesys.FileSystem, revisor *Revisor) (*Transformer, error) {
	if revisor == nil {
		return nil, fmt.Errorf("revisor is nil")
	}

	return &Transformer{
		rv: revisor,
		fs: fs,
	}, nil
}

// Generate label transformer file in the base dir
func (t *Transformer) Generate(base string) error {
	var lt = struct {
		ApiVersion string `json:"apiVersion" yaml:"apiVersion"`
		Kind       string `json:"kind" yaml:"kind"`
		Metadata   struct {
			Name string `json:"name" yaml:"name"`
		} `json:"metadata" yaml:"metadata"`
		Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
		FieldSpecs []types.FieldSpec `json:"fieldSpecs,omitempty" yaml:"fieldSpecs,omitempty"`
	}{
		ApiVersion: "builtin",
		Kind:       "LabelTransformer",
		Metadata: struct {
			Name string `json:"name" yaml:"name"`
		}{
			Name: t.rv.name,
		},
		Labels: t.rv.Labels(),
		FieldSpecs: []types.FieldSpec{
			{Path: "metadata/labels", CreateIfNotPresent: true},
		},
	}

	data, err := yaml.Marshal(lt)
	if err != nil {
		return err
	}

	path := filepath.Join(base, t.rv.LabelsFile())
	if err := t.fs.WriteFile(path, data); err != nil {
		return err
	}

	return nil
}
