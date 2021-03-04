package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/engine"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Kubernetes objects previous applied",
	RunE:  deleteCmdRun,
}

var (
	deleteName         string
	deleteTimeout      time.Duration
	deleteCfgNamespace string
)

func init() {
	deleteCmd.Flags().StringVarP(&deleteName, "name", "", "", "name of the kustomization to be deleted")
	deleteCmd.Flags().StringVarP(&deleteCfgNamespace, "gc-namespace", "", "default", "namespace where the GC snapshot ConfigMap is")
	deleteCmd.Flags().DurationVar(&deleteTimeout, "timeout", 5*time.Minute, "timeout for this operation")

	rootCmd.AddCommand(deleteCmd)
}

func deleteCmdRun(cmd *cobra.Command, args []string) error {
	revisor, err := engine.NewRevisior(group, deleteName, "none")
	if err != nil {
		return err
	}

	gc, err := engine.NewGarbageCollector(revisor, deleteTimeout, engine.NewKubectlExecutor(kubectl, nil))
	if err != nil {
		return err
	}

	write := func(obj string) {
		if !strings.Contains(obj, "No resources found") {
			fmt.Println(obj)
		}
	}

	err = gc.Cleanup(deleteCfgNamespace, write)
	if err != nil {
		return err
	}

	return nil
}
