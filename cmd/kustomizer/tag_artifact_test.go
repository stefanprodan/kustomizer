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

func TestTag(t *testing.T) {
	g := NewWithT(t)
	id := randStringRunes(5)
	tag := "v1.0.0"
	artifact := fmt.Sprintf("oci://%s/%s:%s", registryHost, id, tag)

	err := createNamespace(id)
	g.Expect(err).NotTo(HaveOccurred())

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("push artifact", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"push artifact %s -k %s",
			artifact,
			dir,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})

	t.Run("tag artifact", func(t *testing.T) {
		tag = "v2.0.0"
		output, err := executeCommand(fmt.Sprintf(
			"tag artifact %s %s",
			artifact,
			tag,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(tag))
	})

	t.Run("pull artifact", func(t *testing.T) {
		artifact = fmt.Sprintf("oci://%s/%s:%s", registryHost, id, tag)
		output, err := executeCommand(fmt.Sprintf(
			"pull artifact %s",
			artifact,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})
}
