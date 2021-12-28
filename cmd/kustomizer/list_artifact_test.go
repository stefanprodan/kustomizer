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

func TestListArtifact(t *testing.T) {
	g := NewWithT(t)
	id := randStringRunes(5)

	ver1 := "v1.0.0"
	artifact1 := fmt.Sprintf("oci://%s/%s:%s", registryHost, id, ver1)
	dir1, err := makeTestDir(id+"1", testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	ver2 := "v2.0.0-rc.1"
	artifact2 := fmt.Sprintf("oci://%s/%s:%s", registryHost, id, ver2)
	dir2, err := makeTestDir(id+"2", testManifests(id, id, true))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("push artifacts", func(t *testing.T) {
		_, err = executeCommand(fmt.Sprintf(
			"push artifact %s -k %s",
			artifact1,
			dir1,
		))

		g.Expect(err).NotTo(HaveOccurred())

		_, err = executeCommand(fmt.Sprintf(
			"push artifact %s -k %s",
			artifact2,
			dir2,
		))

		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("lists all versions", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"list artifact oci://%s/%s",
			registryHost,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(ver1))
		g.Expect(output).To(MatchRegexp(ver2))
	})

	t.Run("lists prereleases", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"list artifact oci://%s/%s --semver 2.x-0",
			registryHost,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(Not(MatchRegexp(ver1)))
		g.Expect(output).To(MatchRegexp(ver2))
	})

}
