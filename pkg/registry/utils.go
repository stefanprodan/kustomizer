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

package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
)

const URLPrefix = "oci://"

func ParseURL(ociURL string) (string, error) {
	if !strings.HasPrefix(ociURL, URLPrefix) {
		return "", fmt.Errorf("URL must be in format 'oci://<domain>/<org>/<repo>:<tag>'")
	}

	url := strings.TrimPrefix(ociURL, URLPrefix)
	if _, err := name.ParseReference(url); err != nil {
		return "", fmt.Errorf("'%s' invalid: %w", ociURL, err)
	}
	return url, nil
}

func craneOptions(ctx context.Context) []crane.Option {
	return []crane.Option{
		crane.WithContext(ctx),
		crane.WithUserAgent("kustomizer/v1"),
		crane.WithPlatform(&gcrv1.Platform{
			Architecture: "none",
			OS:           "none",
		}),
	}
}
