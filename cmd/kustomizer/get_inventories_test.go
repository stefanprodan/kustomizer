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

func TestGetInventories(t *testing.T) {
	g := NewWithT(t)
	id := "get-" + randStringRunes(5)
	inventory := fmt.Sprintf("inv-%s", id)

	err := createNamespace(id)
	g.Expect(err).NotTo(HaveOccurred())

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("creates objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"apply --inventory-name %s -k %s --inventory-namespace %s",
			inventory,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})

	t.Run("lists inventories", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"get inventories --namespace %s",
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(inventory))
	})
}
