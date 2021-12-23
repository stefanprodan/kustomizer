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

func TestBuild(t *testing.T) {
	g := NewWithT(t)
	id := randStringRunes(5)

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	patchDir, err := makeTestDir(
		"patch"+id,
		[]TestFile{
			{
				Name: "patch.yaml",
				Body: `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
commonAnnotations:
  test: test-annotation
patches:
  - target:
      kind: ConfigMap
    patch: |
      - op: add
        path: /data/patch
        value:
          test-patch
`,
			},
		},
	)
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("builds objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"build inv %s -f %s -n %s -o yaml",
			id,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})

	t.Run("patch objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"build inv %s -f %s -n %s -p %s -o yaml",
			id,
			dir,
			id,
			patchDir+"/patch.yaml",
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp("test-annotation"))
		g.Expect(output).To(MatchRegexp("test-patch"))
	})
}
