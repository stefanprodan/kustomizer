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
	"fmt"
)

const (
	VersionAnnotation    = "kustomizer.dev/version"
	ChecksumAnnotation   = "kustomizer.dev/checksum"
	CreatedAnnotation    = "kustomizer.dev/created"
	EncryptedAnnotation  = "kustomizer.dev/encrypted"
	AgeEncryptionVersion = "age-encryption.org/v1"
)

type Metadata struct {
	Version   string `json:"version"`
	Checksum  string `json:"checksum"`
	Created   string `json:"created"`
	Encrypted string `json:"encrypted,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

func (m *Metadata) ToAnnotations() map[string]string {
	annotations := map[string]string{
		VersionAnnotation:  m.Version,
		ChecksumAnnotation: m.Checksum,
		CreatedAnnotation:  m.Created,
	}

	if m.Encrypted != "" {
		annotations[EncryptedAnnotation] = m.Encrypted
	}
	return annotations
}

func GetMetadata(annotations map[string]string) (*Metadata, error) {
	version, ok := annotations[VersionAnnotation]
	if !ok {
		return nil, fmt.Errorf("'%s' annotation not found", VersionAnnotation)
	}

	checksum, ok := annotations[ChecksumAnnotation]
	if !ok {
		return nil, fmt.Errorf("'%s' annotation not found", ChecksumAnnotation)
	}

	created, ok := annotations[CreatedAnnotation]
	if !ok {
		return nil, fmt.Errorf("'%s' annotation not found", CreatedAnnotation)
	}

	m := Metadata{
		Version:  version,
		Checksum: checksum,
		Created:  created,
	}

	if encrypted, ok := annotations[EncryptedAnnotation]; ok {
		m.Encrypted = encrypted
	}

	return &m, nil
}
