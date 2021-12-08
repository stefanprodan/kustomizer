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

func TestDelete(t *testing.T) {
	g := NewWithT(t)
	id := "del-" + randStringRunes(5)
	inventory := fmt.Sprintf("inv-%s", id)
	configMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}

	err := createNamespace(id)
	g.Expect(err).NotTo(HaveOccurred())

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("creates objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"apply -i %s -k %s --inventory-namespace %s",
			inventory,
			dir,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(id))
	})

	t.Run("deletes objects", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"delete -i %s --inventory-namespace %s",
			inventory,
			id,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp("deleted"))

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: id,
			},
		}

		err = envTestClient.Get(context.Background(), client.ObjectKeyFromObject(secret), secret)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: id,
			},
		}

		err = envTestClient.Get(context.Background(), client.ObjectKeyFromObject(configMap), configMap)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      inventory,
				Namespace: id,
			},
		}

		err = envTestClient.Get(context.Background(), client.ObjectKeyFromObject(configMap), configMap)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
}
