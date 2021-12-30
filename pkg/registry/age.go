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
	"bytes"
	"io"
	"os"

	"filippo.io/age"
	"filippo.io/age/armor"
)

func ParseAgeRecipients(filePath string) ([]age.Recipient, error) {
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return age.ParseRecipients(f)
	}

	return nil, nil
}

func ParseAgeIdentities(filePath string) ([]age.Identity, error) {
	var identities []age.Identity
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return age.ParseIdentities(f)
	}

	return identities, nil
}

func encrypt(data []byte, recipients []age.Recipient) ([]byte, error) {
	buffer := &bytes.Buffer{}
	aw := armor.NewWriter(buffer)
	w, err := age.Encrypt(aw, recipients...)
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(data); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	if err := aw.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func decrypt(data []byte, identities []age.Identity) ([]byte, error) {
	src := bytes.NewReader(data)
	ar := armor.NewReader(src)
	r, err := age.Decrypt(ar, identities...)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	if _, err := io.Copy(&b, r); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
