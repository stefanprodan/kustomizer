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
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stefanprodan/kustomizer/v2/pkg/registry"
)

var listArtifactCmd = &cobra.Command{
	Use:     "artifact",
	Aliases: []string{"artifacts"},
	Short:   "List the versions of an OCI artifact.",
	Long: `The list command fetches the tags of the specified OCI artifact from its image repository.
If a semantic version condition is specified, the tags are filtered and ordered by semver.
For private registries, the list command uses the credentials from '~/.docker/config.json'.`,
	Example: `  kustomizer list artifacts <oci repository url> --semver <condition>

  # List all versions ordered by semver
  kustomizer list artifacts oci://docker.io/user/repo --semver="*"

  # List all versions including prerelease ordered by semver
  kustomizer list artifacts oci://docker.io/user/repo --semver=">0.0.0-0"

  # List all versions in the 1.0 range
  kustomizer list artifacts oci://docker.io/user/repo --semver="~1.0"

  # List all versions in the 1.0 range including prerelease
  kustomizer list artifacts oci://docker.io/user/repo --semver="~1.0-0"
`,
	RunE: runListArtifactCmd,
}

type listArtifactFlags struct {
	semverExp string
}

var listArtifactArgs listArtifactFlags

func init() {
	listArtifactCmd.Flags().StringVar(&listArtifactArgs.semverExp, "semver", "",
		"Filter the results based on a semantic version constraint e.g. '1.x'.")
	listCmd.AddCommand(listArtifactCmd)
}

func runListArtifactCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an artifact repository e.g. 'oci://docker.io/user/repo'")
	}

	url, err := registry.ParseRepositoryURL(args[0])
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	tags, err := registry.List(ctx, url)
	if err != nil {
		return fmt.Errorf("pulling %s failed: %w", url, err)
	}

	var rows [][]string

	if exp := listArtifactArgs.semverExp; exp != "" {
		c, err := semver.NewConstraint(exp)
		if err != nil {
			return fmt.Errorf("semver '%s' parse error: %w", exp, err)
		}

		var matchingVersions []*semver.Version
		for _, t := range tags {
			v, err := semver.NewVersion(t)
			if err != nil {
				continue
			}

			if c != nil && !c.Check(v) {
				continue
			}

			matchingVersions = append(matchingVersions, v)
		}

		sort.Sort(sort.Reverse(semver.Collection(matchingVersions)))

		for _, ver := range matchingVersions {
			row := []string{ver.String(), fmt.Sprintf("%s:%s", url, ver.Original())}
			rows = append(rows, row)
		}
	} else {
		for _, tag := range tags {
			// exclude cosign signatures
			if !strings.HasSuffix(tag, ".sig") {
				row := []string{tag, fmt.Sprintf("%s:%s", url, tag)}
				rows = append(rows, row)
			}
		}
	}

	printTable(rootCmd.OutOrStdout(), []string{"version", "url"}, rows)

	return nil
}
