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
	"github.com/stefanprodan/kustomizer/pkg/config"
)

var configInit = &cobra.Command{
	Use:   "init",
	Short: "Init writes a config file with default values at '$HOME/.kustomizer/config'.",
	RunE:  runConfigInitCmd,
}

func init() {
	configCmd.AddCommand(configInit)
}

func runConfigInitCmd(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return err
	}

	c := config.NewConfig()
	if err := c.Write(""); err != nil {
		return err
	}

	logger.Println("config written to", cfgPath)
	return nil
}
