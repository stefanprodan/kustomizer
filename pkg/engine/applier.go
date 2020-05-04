package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/api/filesys"
)

type Applier struct {
	rv      *Revisor
	fs      filesys.FileSystem
	timeout time.Duration
}

func NewApplier(fs filesys.FileSystem, revisor *Revisor, timeout time.Duration) (*Applier, error) {
	if revisor == nil {
		return nil, fmt.Errorf("revisor is nil")
	}

	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, fmt.Errorf("kubectl not found")
	}

	return &Applier{
		rv:      revisor,
		fs:      fs,
		timeout: timeout,
	}, nil
}

func (a *Applier) Run(filePath string, dryRun bool) error {
	if !a.fs.Exists(filePath) {
		return fmt.Errorf("%s not found", filePath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.timeout+time.Second)
	defer cancel()

	command := fmt.Sprintf("kubectl apply -f %s --timeout=%s", filePath, a.timeout.String())
	if dryRun {
		command = fmt.Sprintf("%s --dry-run=client", command)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	c := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	c.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	c.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := c.Run(); err != nil {
		return err
	} else {
		return nil
	}
}

type ObjKey struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

func (a *Applier) Extract(filePath string) error {
	data, err := a.fs.ReadFile(filePath)
	if err != nil {
		return err
	}

	resources := map[ObjKey]*unstructured.Unstructured{}

	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 2048)
	for {
		var obj unstructured.Unstructured
		err := reader.Decode(&obj)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if obj.IsList() {
			err := obj.EachListItem(func(item runtime.Object) error {
				castItem := item.(*unstructured.Unstructured)
				resources[ObjKey{
					APIVersion: castItem.GetAPIVersion(),
					Kind:       castItem.GetKind(),
					Name:       castItem.GetName(),
					Namespace:  castItem.GetNamespace(),
				}] = castItem
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			resources[ObjKey{
				APIVersion: obj.GetAPIVersion(),
				Kind:       obj.GetKind(),
				Name:       obj.GetName(),
				Namespace:  obj.GetNamespace(),
			}] = &obj
		}
	}

	for key, _ := range resources {
		fmt.Println(key)
	}

	return nil
}
