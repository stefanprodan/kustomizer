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
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/mattn/go-shellwords"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	tmpDir        string
	envTestClient client.WithWatch
	registryHost  string
)

func TestMain(m *testing.M) {
	regURL, err := startTestRegistry()
	if err != nil {
		panic(err)
	}
	registryHost = regURL

	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}

	user, err := testEnv.ControlPlane.AddUser(envtest.User{
		Name:   "envtest-admin",
		Groups: []string{"system:masters"},
	}, nil)
	if err != nil {
		panic(err)
	}

	kubeConfig, err := user.KubeConfig()
	if err != nil {
		panic(err)
	}

	tmpDir, err = os.MkdirTemp("", "kustomizer")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFilename := filepath.Join(tmpDir, "kubeconfig-"+time.Nanosecond.String())
	if err := os.WriteFile(tmpFilename, kubeConfig, 0644); err != nil {
		panic(err)
	}

	envTestClient, err = client.NewWithWatch(cfg, client.Options{
		Scheme: newScheme(),
	})
	if err != nil {
		panic(err)
	}

	kubeconfigArgs.KubeConfig = &tmpFilename

	code := m.Run()

	testEnv.Stop()

	os.Exit(code)
}

func startTestRegistry() (string, error) {
	port, err := getFreePort()
	if err != nil {
		return "", err
	}

	registryHost = fmt.Sprintf("localhost:%d", port)
	config := &configuration.Configuration{}
	config.Log.Level = configuration.Loglevel("error")
	config.Log.AccessLog.Disabled = true
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	if err != nil {
		return "", err
	}

	go dockerRegistry.ListenAndServe()

	return registryHost, nil
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

type TestFile struct {
	Name string
	Body string
}

func makeTestDir(name string, files []TestFile) (string, error) {
	dir := filepath.Join(tmpDir, name)
	_ = os.RemoveAll(dir)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return dir, err
	}

	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.Name), []byte(file.Body), 0644); err != nil {
			return dir, err
		}
	}
	return dir, nil
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func createNamespace(name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	return envTestClient.Create(context.Background(), namespace)
}

func executeCommand(cmd string) (string, error) {
	defer resetCmdArgs()
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)

	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)

	logger.stderr = rootCmd.ErrOrStderr()

	_, err = rootCmd.ExecuteC()
	result := buf.String()

	return result, err
}

func resetCmdArgs() {
	applyInventoryArgs = applyInventoryFlags{}
	buildInventoryArgs = buildInventoryFlags{}
}

var testManifests = func(name, namespace string, immutable bool) []TestFile {
	return []TestFile{
		{
			Name: "kustomization.yaml",
			Body: fmt.Sprintf(`---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: "%[1]s"
resources:
  - config.yaml
  - secret.yaml
  - cron.yaml
`, namespace),
		},
		{
			Name: "config.yaml",
			Body: fmt.Sprintf(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: "%[1]s"
data:
  key: "test"
`, name),
		},
		{
			Name: "secret.yaml",
			Body: fmt.Sprintf(`---
apiVersion: v1
kind: Secret
metadata:
  name: "%[1]s"
immutable: %[2]t
stringData:
  key: "%[3]d"
`, name, immutable, time.Now().UnixNano()),
		},
		{
			Name: "cron.yaml",
			Body: fmt.Sprintf(`---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: "%[1]s"
spec:
  schedule: "*/30 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
          - name: version
            image: ghcr.io://ghcr.io/stefanprodan/podinfo:v6.0.0
            imagePullPolicy: IfNotPresent
            command:
            - ./podinfo
            - --version
`, name),
		},
	}
}
