package ksync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"sigs.k8s.io/kustomize/api/filesys"
)

type Applier struct {
	rv *Revisor
	fs filesys.FileSystem
}

func NewApplier(fs filesys.FileSystem, revisor *Revisor) (*Applier, error) {
	if revisor == nil {
		return nil, fmt.Errorf("revisor is nil")
	}

	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, fmt.Errorf("kubectl not found")
	}

	return &Applier{
		rv: revisor,
		fs: fs,
	}, nil
}

func (a *Applier) Apply(ctx context.Context, filePath string, dryRun bool) error {
	command := fmt.Sprintf("kubectl apply -f %s", filePath)
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

func (a *Applier) Prune(ctx context.Context, dryRun bool, write func(string)) error {
	namespacedKinds, err := a.listKinds(ctx, true)
	if err != nil {
		return err
	}

	for _, kind := range strings.Split(namespacedKinds, ",") {
		o, err := a.delete(ctx, kind, a.rv.PrevSelectors(), dryRun)
		if err != nil {
			return err
		}
		if !strings.Contains(o, "No resources found") {
			write(o)
		}
	}

	clusterKinds, err := a.listKinds(ctx, false)
	if err != nil {
		return err
	}

	for _, kind := range strings.Split(clusterKinds, ",") {
		o, err := a.delete(ctx, kind, a.rv.PrevSelectors(), dryRun)
		if err != nil {
			return err
		}
		if !strings.Contains(o, "No resources found") {
			write(o)
		}
	}
	return nil
}

func (a *Applier) delete(ctx context.Context, kind string, selector string, dryRun bool) (string, error) {
	cmd := fmt.Sprintf("kubectl delete %s --all-namespaces -l %s", kind, selector)
	if dryRun {
		cmd = fmt.Sprintf("%s --dry-run=server", cmd)
	}

	command := exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s", string(output))
	} else {
		return strings.TrimSuffix(string(output), "\n"), nil
	}
}

func (a *Applier) listKinds(ctx context.Context, namespaced bool) (string, error) {
	exclude := `grep -vE "(events|nodes)"`
	flat := `tr "\n" "," | sed -e 's/,$//'`
	cmd := fmt.Sprintf(`kubectl api-resources --cached=true --namespaced=%t --verbs=delete -o name | %s | %s`,
		namespaced, exclude, flat)
	command := exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s", string(output))
	} else {
		return strings.TrimSuffix(string(output), "\n"), nil
	}
}
