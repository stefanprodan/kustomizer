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
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var configView = &cobra.Command{
	Use: "view",
	Short: "Display the config values from '$HOME/.kustomizer/config'. " +
		"If no config file is found, the default in-memory values are displayed.",
	RunE: runConfigViewCmd,
}

func init() {
	configCmd.AddCommand(configView)
}

func runConfigViewCmd(cmd *cobra.Command, args []string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	rootCmd.Println(string(data))
	return nil
}
