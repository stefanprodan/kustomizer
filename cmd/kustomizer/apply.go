package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/filesys"

	"github.com/stefanprodan/kustomizer/pkg/ksync"
)

var applyCmd = &cobra.Command{
	Use:   "apply [path]",
	Short: "Apply kustomization and prune previous applied Kubernetes objects",
	RunE:  applyCmdRun,
}

var (
	group        string
	name         string
	nextRevision string
	prevRevision string
	dryRun       bool
)

func init() {
	applyCmd.Flags().StringVar(&group, "group", "kustomizer", "group")
	applyCmd.Flags().StringVarP(&name, "name", "n", "", "name")
	applyCmd.Flags().StringVarP(&nextRevision, "revision", "r", "", "revision")
	applyCmd.Flags().StringVarP(&prevRevision, "prev-revision", "p", "", "previous revision")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run")

	rootCmd.AddCommand(applyCmd)
}

func applyCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("path is required")
	}
	base := args[0]
	fs := filesys.MakeFsOnDisk()

	revisor, err := ksync.NewRevisior(group, name, nextRevision, prevRevision)
	if err != nil {
		return err
	}

	transformer, err := ksync.NewTransformer(fs, revisor)
	if err != nil {
		return err
	}
	err = transformer.Generate(base)
	if err != nil {
		return err
	}

	generator, err := ksync.NewGenerator(fs, revisor)
	if err != nil {
		return err
	}
	err = generator.Generate(base)
	if err != nil {
		return err
	}

	builder, err := ksync.NewBuilder(fs)
	if err != nil {
		return err
	}

	manifests := filepath.Join(base, revisor.ManifestFile())
	err = builder.Generate(base, manifests)
	if err != nil {
		return err
	}

	applier, err := ksync.NewApplier(fs, revisor)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = applier.Apply(ctx, manifests, dryRun)
	if err != nil {
		return err
	}

	write := func(obj string) {
		fmt.Println(obj)
	}

	err = applier.Prune(ctx, dryRun, write)
	if err != nil {
		return err
	}

	err = os.Remove(manifests)
	if err != nil {
		return err
	}

	return nil
}
