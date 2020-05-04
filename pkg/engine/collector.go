package engine

import (
	"context"
	"fmt"
	"io/ioutil"
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

func (gc *GarbageCollector) Run(manifestsFile string, cfgNamespace string, write func(string)) error {
	data, err := ioutil.ReadFile(manifestsFile)
	if err != nil {
		return err
	}

	newSnapshot, err := NewSnapshot(data, gc.rv.revision)
	if err != nil {
		return err
	}

	firstTime := false
	cfg, err := gc.getSnapshot(cfgNamespace)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			firstTime = true
		} else {
			return err
		}
	}

	if !firstTime {
		oldSnapshot, err := NewSnapshotFromConfigMap(cfg)
		if err != nil {
			return err
		}

		if newSnapshot.Revision != oldSnapshot.Revision {
			err := gc.prune(*oldSnapshot, false, write)
			if err != nil {
				return err
			}
		}
	}

	newCfg, err := newSnapshot.ToConfigMap(gc.rv.SnapshotName(), cfgNamespace)
	if err != nil {
		return err
	}

	msg, err := gc.applySnapshot(newCfg)
	if err != nil {
		return err
	}

	write(msg)
	return nil
}

func (gc *GarbageCollector) prune(snapshot Snapshot, dryRun bool, write func(string)) error {
	selector := gc.rv.PrevSelectors(snapshot.Revision)
	for ns, kinds := range snapshot.NamespacedKinds() {
		for _, kind := range kinds {
			if output, err := gc.deleteByKind(kind, ns, selector, dryRun, gc.timeout); err != nil {
				write(err.Error())
			} else {
				write(output)
			}
		}
	}

	for _, kind := range snapshot.NonNamespacedKinds() {
		if output, err := gc.deleteByKind(kind, "", selector, dryRun, gc.timeout); err != nil {
			write(err.Error())
		} else {
			write(output)
		}
	}

	return nil
}

func (gc *GarbageCollector) deleteByKind(kind string, namespace string, selector string, dryRun bool, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Second)
	defer cancel()

	cmd := fmt.Sprintf("kubectl delete %s -l %s", kind, selector)
	if namespace != "" {
		cmd = fmt.Sprintf("%s -n=%s", cmd, namespace)
	}
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

func (gc *GarbageCollector) getSnapshot(cfgNamespace string) (string, error) {
	cmd := fmt.Sprintf("kubectl -n %s get configmap %s -oyaml", cfgNamespace, gc.rv.SnapshotName())
	command := exec.Command("/bin/sh", "-c", cmd)
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s", string(output))
	} else {
		return strings.TrimSuffix(string(output), "\n"), nil
	}
}

func (gc *GarbageCollector) applySnapshot(cfg string) (string, error) {
	cmd := fmt.Sprintf("echo '%s' |kubectl apply -f-", cfg)
	command := exec.Command("/bin/sh", "-c", cmd)
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s", string(output))
	} else {
		return strings.TrimSuffix(string(output), "\n"), nil
	}
}
