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
	"os"
	"path/filepath"

	"filippo.io/age"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

func Push(ctx context.Context, url string, data []byte, meta *Metadata, recipients []age.Recipient) (string, error) {
	ref, err := name.ParseReference(url)
	if err != nil {
		return "", fmt.Errorf("parsing refernce failed: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "oci")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tarFile := filepath.Join(tmpDir, "all.tar")
	dataFile := "all.yaml"

	if len(recipients) > 0 {
		meta.Encrypted = AgeEncryptionVersion
		encData, err := encrypt(data, recipients)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt data with age: %w", err)
		}

		dataFile = "all.yaml.age"
		data = encData
	}

	if err := tarContent(tarFile, dataFile, data); err != nil {
		return "", err
	}

	img, err := crane.Append(empty.Image, tarFile)
	if err != nil {
		return "", fmt.Errorf("appending content failed: %w", err)
	}

	img = mutate.Annotations(img, meta.ToAnnotations()).(gcrv1.Image)

	if err := crane.Push(img, url, craneOptions(ctx)...); err != nil {
		return "", fmt.Errorf("pushing image failed: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing digest failed: %w", err)
	}

	return ref.Context().Digest(digest.String()).String(), nil
}
