package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/filesys"

	"github.com/stefanprodan/kustomizer/pkg/engine"
)

var applyCmd = &cobra.Command{
	Use:   "apply [path]",
	Short: "Run kustomization and prune previous applied Kubernetes objects",
	RunE:  applyCmdRun,
}

var (
	group        string
	name         string
	nextRevision string
	prevRevision string
	dryRun       bool
	timeout      time.Duration
)

func init() {
	applyCmd.Flags().StringVar(&group, "group", "kustomizer", "group")
	applyCmd.Flags().StringVarP(&name, "name", "n", "", "name")
	applyCmd.Flags().StringVarP(&nextRevision, "revision", "r", "", "revision")
	applyCmd.Flags().StringVarP(&prevRevision, "prev-revision", "p", "", "previous revision")
	applyCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout for this operation")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run")

	rootCmd.AddCommand(applyCmd)
}

func applyCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path is required")
	}
	base := args[0]
	fs := filesys.MakeFsOnDisk()

	revisor, err := engine.NewRevisior(group, name, nextRevision, prevRevision)
	if err != nil {
		return err
	}

	transformer, err := engine.NewTransformer(fs, revisor)
	if err != nil {
		return err
	}
	err = transformer.Generate(base)
	if err != nil {
		return err
	}

	generator, err := engine.NewGenerator(fs, revisor)
	if err != nil {
		return err
	}
	err = generator.Generate(base)
	if err != nil {
		return err
	}

	builder, err := engine.NewBuilder(fs)
	if err != nil {
		return err
	}

	manifests := filepath.Join(base, revisor.ManifestFile())
	err = builder.Generate(base, manifests)
	if err != nil {
		return err
	}

	applier, err := engine.NewApplier(fs, revisor, timeout)
	if err != nil {
		return err
	}

	err = applier.Run(manifests, dryRun)
	if err != nil {
		return err
	}

	gc, err := engine.NewGarbageCollector(revisor, timeout)
	if err != nil {
		return err
	}

	write := func(obj string) {
		fmt.Println(obj)
	}

	err = gc.Prune(dryRun, write)
	if err != nil {
		return err
	}

	err = os.Remove(manifests)
	if err != nil {
		return err
	}

	return nil
}
