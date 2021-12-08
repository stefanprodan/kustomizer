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
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

func TestApply(t *testing.T) {
	g := NewWithT(t)
	id := "apply-" + randStringRunes(5)
	inventory := fmt.Sprintf("inv-%s", id)
	configMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}

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

	t.Run("labels objects", func(t *testing.T) {
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: id,
			},
		}

		err = envTestClient.Get(context.Background(), client.ObjectKeyFromObject(configMap), configMap)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(configMap.GetLabels()).To(HaveKeyWithValue("inventory.kustomizer.dev/name", "inv-"+id))
		g.Expect(configMap.GetLabels()).To(HaveKeyWithValue("inventory.kustomizer.dev/namespace", id))
	})

	t.Run("prunes objects", func(t *testing.T) {
		dir, err := makeTestDir(id, testManifests(id+"-1", id, false))
		g.Expect(err).NotTo(HaveOccurred())

		output, err := executeCommand(fmt.Sprintf(
			"apply --inventory-name %s -k %s --inventory-namespace %s --prune",
			inventory,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: id,
			},
		}

		err = envTestClient.Get(context.Background(), client.ObjectKeyFromObject(secret), secret)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id + "-1",
				Namespace: id,
			},
		}

		err = envTestClient.Get(context.Background(), client.ObjectKeyFromObject(secret), secret)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(secret.GetLabels()).To(HaveKeyWithValue("inventory.kustomizer.dev/name", "inv-"+id))
		g.Expect(secret.GetLabels()).To(HaveKeyWithValue("inventory.kustomizer.dev/namespace", id))
	})

	t.Run("waits for objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"apply --inventory-name %s -k %s --inventory-namespace %s --prune --wait",
			inventory,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp("waiting"))
	})

	t.Run("recreates immutable objects", func(t *testing.T) {
		dir, err := makeTestDir(id, testManifests(id, id, true))
		g.Expect(err).NotTo(HaveOccurred())

		output, err := executeCommand(fmt.Sprintf(
			"apply --inventory-name %s -k %s --inventory-namespace %s --prune",
			inventory,
			dir,
			id,
		))

		dir, err = makeTestDir(id, testManifests(id, id, true))
		g.Expect(err).NotTo(HaveOccurred())

		output, err = executeCommand(fmt.Sprintf(
			"apply --inventory-name inv-%s -k %s --inventory-namespace %s",
			id,
			dir,
			id,
		))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(MatchRegexp("immutable"))

		output, err = executeCommand(fmt.Sprintf(
			"apply --inventory-name %s -k %s --inventory-namespace %s --force",
			inventory,
			dir,
			id,
		))
		g.Expect(err).ToNot(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp("created"))
	})
}
