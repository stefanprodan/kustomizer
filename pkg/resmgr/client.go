/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package resmgr

import (
	"fmt"
	"runtime"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func newScheme() *apiruntime.Scheme {
	scheme := apiruntime.NewScheme()
	_ = apiextensionsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}

func newKubeClient(kubeConfigPath string, kubeContext string) (client.WithWatch, error) {
	cfg, err := newKubeConfig(kubeConfigPath, kubeContext)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client initialization failed: %w", err)
	}

	kubeClient, err := client.NewWithWatch(cfg, client.Options{
		Scheme: newScheme(),
	})
	if err != nil {
		return nil, fmt.Errorf("kubernetes client initialization failed: %w", err)
	}

	return kubeClient, nil
}

func newKubeStatusPoller(kubeConfigPath string, kubeContext string) (*polling.StatusPoller, error) {
	kubeConfig, err := newKubeConfig(kubeConfigPath, kubeContext)
	if err != nil {
		return nil, err
	}

	restMapper, err := apiutil.NewDynamicRESTMapper(kubeConfig)
	if err != nil {
		return nil, err
	}
	c, err := client.New(kubeConfig, client.Options{Mapper: restMapper})
	if err != nil {
		return nil, err
	}

	return polling.NewStatusPoller(c, restMapper), nil
}

func newKubeConfig(kubeConfigPath string, kubeContext string) (*rest.Config, error) {
	configFiles := splitKubeConfigPath(kubeConfigPath)
	configOverrides := clientcmd.ConfigOverrides{}

	if len(kubeContext) > 0 {
		configOverrides.CurrentContext = kubeContext
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: configFiles},
		&configOverrides,
	).ClientConfig()

	if err != nil {
		return nil, fmt.Errorf("kubeconfig load failed: %w", err)
	}

	cfg.QPS = 50
	cfg.Burst = 100

	return cfg, nil
}

func splitKubeConfigPath(path string) []string {
	var sep string
	switch runtime.GOOS {
	case "windows":
		sep = ";"
	default:
		sep = ":"
	}
	return strings.Split(path, sep)
}
