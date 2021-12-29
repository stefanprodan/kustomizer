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
	"archive/tar"
	"io"
	"os"
	"strings"
)

func tarContent(tarPath string, name string, data []byte) error {
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()
	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	header := &tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(data)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(data); err != nil {
		return err
	}

	return nil
}

func untarContent(r io.Reader) (string, error) {
	sb := new(strings.Builder)
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return sb.String(), nil
		case err != nil:
			return "", err
		case header == nil:
			continue
		}

		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(sb, tr); err != nil {
				return "", err
			}
		}
	}
}
