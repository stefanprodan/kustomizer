package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/api/filesys"
	yaml2 "sigs.k8s.io/yaml"
)

type Applier struct {
	fs      filesys.FileSystem
	timeout time.Duration
}

func NewApplier(fs filesys.FileSystem, timeout time.Duration) (*Applier, error) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, fmt.Errorf("kubectl not found")
	}

	return &Applier{
		fs:      fs,
		timeout: timeout,
	}, nil
}

func (a *Applier) Run(manifestPath string, dryRun bool) error {
	if !a.fs.Exists(manifestPath) {
		return fmt.Errorf("%s not found", manifestPath)
	}

	crds, err := a.ExtractCRDs(manifestPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.timeout+time.Second)
	defer cancel()

	if crds != "" {
		args := []string{"apply", "-f", crds, "--timeout", a.timeout.String()}
		if dryRun {
			args = append(args, "--dry-run", "client")
		}

		if err := NewKubectlExecutor(nil).Exec(ctx, args...); err != nil {
			return err
		}
	}

	args := []string{"apply", "-f", manifestPath, "--timeout", a.timeout.String()}
	if dryRun {
		args = append(args, "--dry-run", "client")
	}

	return NewKubectlExecutor(nil).Exec(ctx, args...)
}

func (a *Applier) ExtractCRDs(manifestPath string) (string, error) {
	manifests, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}

	crds := ""
	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifests), 2048)
	for {
		var obj unstructured.Unstructured
		err := reader.Decode(&obj)
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		if obj.IsList() {
			err := obj.EachListItem(func(item runtime.Object) error {
				return nil
			})
			if err != nil {
				return "", err
			}
		} else {
			if obj.GetKind() == "CustomResourceDefinition" {
				b, err := obj.MarshalJSON()
				if err != nil {
					return "", err
				}

				y, err := yaml2.JSONToYAML(b)
				if err != nil {
					return "", err
				}
				crds += "---\n" + string(y)
			}
		}
	}

	if crds == "" {
		return "", nil
	}

	base := filepath.Dir(manifestPath)
	crdsFile := filepath.Join(base, "extracted-crds.yaml")

	if err := ioutil.WriteFile(crdsFile, []byte(crds), os.ModePerm); err != nil {
		return "", err
	}

	return crdsFile, nil
}
