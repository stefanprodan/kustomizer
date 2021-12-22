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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDiffInventory(t *testing.T) {
	g := NewWithT(t)
	id := "diff-" + randStringRunes(5)

	err := createNamespace(id)
	g.Expect(err).NotTo(HaveOccurred())

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("creates objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"apply inv %s -k %s -n %s",
			id,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})

	t.Run("generates empty diff", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"diff inv %s -k %s -n %s --prune",
			id,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(BeEmpty())
	})

	t.Run("generates created diff", func(t *testing.T) {
		dir, err := makeTestDir(id, testManifests(id+"-1", id, false))
		g.Expect(err).NotTo(HaveOccurred())

		output, err := executeCommand(fmt.Sprintf(
			"diff inv %s -k %s -n %s --prune",
			id,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp("created"))
		g.Expect(output).To(MatchRegexp("deleted"))
	})
	t.Run("generates YAML diff", func(t *testing.T) {
		dir, err := makeTestDir(id, testManifests(id, id, true))
		g.Expect(err).NotTo(HaveOccurred())

		output, err := executeCommand(fmt.Sprintf(
			"diff inv %s -k %s -n %s --prune",
			id,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp("immutable"))
	})
}
