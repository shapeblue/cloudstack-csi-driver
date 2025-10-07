//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//

package cloud

import (
	"fmt"

	gcfg "gopkg.in/gcfg.v1"
)

// Config holds CloudStack connection configuration.
type Config struct {
	APIURL    string
	APIKey    string
	SecretKey string
	VerifySSL bool
	ProjectID string
}

// csConfig wraps the config for the CloudStack cloud provider.
// It is taken from https://github.com/apache/cloudstack-kubernetes-provider
// in order to have the same config in cloudstack-kubernetes-provider
// and in this cloudstack-csi-driver.
type csConfig struct {
	Global struct {
		APIURL      string `gcfg:"api-url"`
		APIKey      string `gcfg:"api-key"`
		SecretKey   string `gcfg:"secret-key"`
		SSLNoVerify bool   `gcfg:"ssl-no-verify"`
		ProjectID   string `gcfg:"project-id"`
		Zone        string `gcfg:"zone"`
	}
}

// ReadConfig reads a config file with a format defined by CloudStack
// Cloud Controller Manager, and returns a CloudStackConfig.
func ReadConfig(configFilePath string) (*Config, error) {
	cfg := &csConfig{}
	if err := gcfg.ReadFileInto(cfg, configFilePath); err != nil {
		return nil, fmt.Errorf("could not parse CloudStack config: %w", err)
	}

	return &Config{
		APIURL:    cfg.Global.APIURL,
		APIKey:    cfg.Global.APIKey,
		ProjectID: cfg.Global.ProjectID,
		SecretKey: cfg.Global.SecretKey,
		VerifySSL: !cfg.Global.SSLNoVerify,
	}, nil
}
