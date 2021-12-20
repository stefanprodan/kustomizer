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

func TestGetInventory(t *testing.T) {
	g := NewWithT(t)
	id := "get-" + randStringRunes(5)

	source := "https://github.com/stefanprodan/kustomizer.git"
	revision := "v2.0.0"

	err := createNamespace(id)
	g.Expect(err).NotTo(HaveOccurred())

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("creates objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"apply inv %s -k %s --namespace %s --source %s --revision %s",
			id,
			dir,
			id,
			source,
			revision,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})

	t.Run("prints inventory details", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"inspect inventory %s --namespace %s",
			id,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(fmt.Sprintf("%s/%s", id, id)))
		g.Expect(output).To(MatchRegexp(source))
		g.Expect(output).To(MatchRegexp(revision))
		g.Expect(output).To(MatchRegexp(fmt.Sprintf("ConfigMap/%s/%s", id, id)))
		g.Expect(output).To(MatchRegexp(fmt.Sprintf("Secret/%s/%s", id, id)))
	})
}
