// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * WebUI Configuration Factory
 */

package factory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

var WebUIConfig *Config

func init() {
	WebUIConfig = &Config{Configuration: &Configuration{CfgPort: 5000}}
}

func GetConfig() *Config {
	return WebUIConfig
}

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return fmt.Errorf("[Configuration] %+v", err)
	} else {
		if yamlErr := yaml.Unmarshal(content, WebUIConfig); yamlErr != nil {
			return fmt.Errorf("[Configuration] %+v", yamlErr)
		}
		if WebUIConfig.Configuration.Mongodb.AuthUrl == "" {
			authUrl := WebUIConfig.Configuration.Mongodb.Url
			WebUIConfig.Configuration.Mongodb.AuthUrl = authUrl
		}
		if WebUIConfig.Configuration.Mongodb.AuthKeysDbName == "" {
			WebUIConfig.Configuration.Mongodb.AuthKeysDbName = "authentication"
		}
		// we dont want Mode5G coming from the helm chart, since
		// there is chance of misconfiguration
		if os.Getenv("CONFIGPOD_DEPLOYMENT") == "4G" {
			fmt.Println("ConfigPod running in 4G deployment")
			WebUIConfig.Configuration.Mode5G = false
		} else {
			// default mode
			fmt.Println("ConfigPod running in 5G deployment")
			WebUIConfig.Configuration.Mode5G = true
		}
	}

	return nil
}
