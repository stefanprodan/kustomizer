package engine

import (
	"crypto/sha1"
	"fmt"
)

type Revisor struct {
	group    string
	name     string
	revision string
}

func NewRevisior(group, name, revision string) (*Revisor, error) {
	if group == "" {
		return nil, fmt.Errorf("group not specified")
	}
	if name == "" {
		return nil, fmt.Errorf("name not specified")
	}
	if revision == "" {
		return nil, fmt.Errorf("revision not specified")
	}

	return &Revisor{
		group:    group,
		name:     name,
		revision: revision,
	}, nil
}

func (r *Revisor) Hash() string {
	gv := fmt.Sprintf("%s-%s", r.group, r.name)
	return fmt.Sprintf("%x", sha1.Sum([]byte(gv)))
}

func (r *Revisor) Labels() map[string]string {
	return map[string]string{
		fmt.Sprintf("%s/name", r.group):     r.name,
		fmt.Sprintf("%s/revision", r.group): r.revision,
	}
}

func (r *Revisor) LabelsFile() string {
	return fmt.Sprintf("%s-labels.yaml", r.Hash())
}

func (r *Revisor) NextSelectors() string {
	return fmt.Sprintf("%s/name=%s,%s/revision=%s", r.group, r.name, r.group, r.revision)
}

func (r *Revisor) PrevSelectors(revision string) string {
	return fmt.Sprintf("%s/name=%s,%s/revision=%s", r.group, r.name, r.group, revision)
}

func (r *Revisor) ManifestFile() string {
	return fmt.Sprintf("%s-manifest.yaml", r.Hash())
}

func (r *Revisor) SnapshotName() string {
	return fmt.Sprintf("%s-snapshot", r.name)
}
