package ksync

import (
	"crypto/sha1"
	"fmt"
)

type Revisor struct {
	group        string
	name         string
	nextRevision string
	prevRevision string
}

func NewRevisior(group, name, nextRevision, prevRevision string) (*Revisor, error) {
	if group == "" {
		return nil, fmt.Errorf("group not specified")
	}
	if name == "" {
		return nil, fmt.Errorf("name not specified")
	}
	if nextRevision == "" {
		return nil, fmt.Errorf("next revision not specified")
	}
	if prevRevision == "" {
		return nil, fmt.Errorf("previous revision not specified")
	}

	return &Revisor{
		group:        group,
		name:         name,
		nextRevision: nextRevision,
		prevRevision: prevRevision,
	}, nil
}

func (r *Revisor) Hash() string {
	gv := fmt.Sprintf("%s-%s-%s-%s", r.group, r.name, r.nextRevision, r.prevRevision)
	return fmt.Sprintf("%x", sha1.Sum([]byte(gv)))
}

func (r *Revisor) Labels() map[string]string {
	return map[string]string{
		fmt.Sprintf("%s/name", r.group):     r.name,
		fmt.Sprintf("%s/revision", r.group): r.nextRevision,
	}
}

func (r *Revisor) LabelsFile() string {
	return fmt.Sprintf("%s-labels.yaml", r.Hash())
}

func (r *Revisor) NextSelectors() string {
	return fmt.Sprintf("%s/name=%s,%s/revision=%s", r.group, r.name, r.group, r.nextRevision)
}

func (r *Revisor) PrevSelectors() string {
	return fmt.Sprintf("%s/name=%s,%s/revision=%s", r.group, r.name, r.group, r.prevRevision)
}

func (r *Revisor) ManifestFile() string {
	return fmt.Sprintf("%s-manifest.yaml", r.Hash())
}
