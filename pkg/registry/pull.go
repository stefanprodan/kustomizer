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
	"crypto/sha256"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
)

func Pull(ctx context.Context, url string) (string, error) {
	if _, err := name.ParseReference(url); err != nil {
		return "", fmt.Errorf("parsing refernce failed: %w", err)
	}

	img, err := crane.Pull(url, craneOptions(ctx)...)
	if err != nil {
		return "", err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return "", err
	}

	meta, err := GetMetadata(manifest.Annotations)
	if err != nil {
		return "", err
	}

	layers, err := img.Layers()
	if err != nil {
		return "", err
	}

	if len(layers) < 1 {
		return "", fmt.Errorf("no layers found in image")
	}

	blob, err := layers[0].Uncompressed()
	if err != nil {
		return "", err
	}

	content, err := untarContent(blob)
	if err != nil {
		return "", err
	}

	if meta.Checksum != fmt.Sprintf("%x", sha256.Sum256([]byte(content))) {
		return "", fmt.Errorf("checksum mismatch")
	}

	return content, nil
}
