/*
Copyright 2021 Stefan Prodan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var tagCmd = &cobra.Command{
	Use:   "tag OCIURL TAG",
	Short: "Tag adds a tag for the specified OCI artifact.",
	Long: `The tag command tags an existing image on the remote container registry.
This command uses the credentials from '~/.docker/config.json'.`,
	Example: `  # Tag an OCI artifact as latest
  kustomizer tag oci://docker.io/user/repo:v1.0.0 latest
`,
	RunE: runTagCmd,
}

func init() {
	rootCmd.AddCommand(tagCmd)
}

func runTagCmd(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("you must specify an artifact name e.g. 'oci://docker.io/user/repo:tag' and a tag")
	}

	url, err := registry.ParseURL(args[0])
	if err != nil {
		return err
	}

	tag := args[1]

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	res, err := registry.Tag(ctx, url, tag)
	if err != nil {
		return fmt.Errorf("tagging %s failed: %w", url, err)
	}

	logger.Println("tagged", res)

	return nil
}
