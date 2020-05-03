package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type GarbageCollector struct {
	rv      *Revisor
	timeout time.Duration
}

func NewGarbageCollector(revisor *Revisor, timeout time.Duration) (*GarbageCollector, error) {
	if revisor == nil {
		return nil, fmt.Errorf("revisor is nil")
	}

	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, fmt.Errorf("kubectl not found")
	}

	return &GarbageCollector{
		rv:      revisor,
		timeout: timeout,
	}, nil
}

func (gc *GarbageCollector) Prune(dryRun bool, write func(string)) error {
	ctx, cancel := context.WithTimeout(context.Background(), gc.timeout+time.Second)
	defer cancel()

	namespacedKinds, err := gc.listKinds(ctx, true)
	if err != nil {
		return err
	}

	for _, kind := range strings.Split(namespacedKinds, ",") {
		o, err := gc.delete(ctx, kind, gc.rv.PrevSelectors(), dryRun)
		if err != nil {
			return err
		}
		if !strings.Contains(o, "No resources found") {
			write(o)
		}
	}

	clusterKinds, err := gc.listKinds(ctx, false)
	if err != nil {
		return err
	}

	for _, kind := range strings.Split(clusterKinds, ",") {
		o, err := gc.delete(ctx, kind, gc.rv.PrevSelectors(), dryRun)
		if err != nil {
			return err
		}
		if !strings.Contains(o, "No resources found") {
			write(o)
		}
	}
	return nil
}

func (gc *GarbageCollector) delete(ctx context.Context, kind string, selector string, dryRun bool) (string, error) {
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

func (gc *GarbageCollector) listKinds(ctx context.Context, namespaced bool) (string, error) {
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
