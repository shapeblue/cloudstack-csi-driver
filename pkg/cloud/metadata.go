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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

const (
	cloudInitInstanceFilePath = "/run/cloud-init/instance-data.json"
	ignitionMetadataFilePath  = "/run/metadata/coreos"
	cloudStackCloudName       = "cloudstack"
)

// metadataInstanceID tries to find the instance ID from either the environment variable NODE_ID,
// or cloud-init or ignition metadata. Returns empty string if not found in any of these sources.
func (c *client) metadataInstanceID(ctx context.Context) string {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Attempting to retrieve metadata from envvar NODE_ID")

	// Try a NODE_ID environment variable
	if envNodeID := os.Getenv("NODE_ID"); envNodeID != "" {
		logger.Info("Found CloudStack VM ID from envvar NODE_ID", "nodeID", envNodeID)

		return envNodeID
	}

	// Try cloud-init
	logger.V(4).Info("Environment variable NODE_ID not found, trying with cloud-init")
	if _, err := os.Stat(cloudInitInstanceFilePath); err == nil {
		logger.V(4).Info("File " + cloudInitInstanceFilePath + " exists")
		ciData, err := c.readCloudInit(ctx, cloudInitInstanceFilePath)
		if err != nil {
			logger.Error(err, "Cannot read cloud-init instance data")
		} else if ciData.V1.InstanceID != "" {
			logger.Info("Found CloudStack VM ID from cloud-init", "nodeID", ciData.V1.InstanceID)

			return ciData.V1.InstanceID
		}
		logger.Error(nil, "cloud-init instance ID is not provided")
	} else if os.IsNotExist(err) {
		logger.V(4).Info("File " + cloudInitInstanceFilePath + " does not exist")
	} else {
		logger.Error(err, "Cannot read file "+cloudInitInstanceFilePath)
	}

	// Try Ignition (CoreOS / Flatcar)
	logger.V(4).Info("Trying with ignition")
	if _, err := os.Stat(ignitionMetadataFilePath); err == nil {
		logger.V(4).Info("File " + ignitionMetadataFilePath + " exists")
		instanceID, err := c.readIgnition(ctx, ignitionMetadataFilePath)
		if err != nil {
			logger.Error(err, "Cannot read ignition metadata")
		} else if instanceID != "" {
			logger.Info("Found CloudStack VM ID from ignition", "nodeID", instanceID)

			return instanceID
		}
		logger.Error(nil, "Failed to find instance ID in ignition metadata")
	} else if os.IsNotExist(err) {
		logger.V(4).Info("File " + ignitionMetadataFilePath + " does not exist")
	} else {
		logger.Error(err, "Cannot read file "+ignitionMetadataFilePath)
	}

	logger.Info("CloudStack VM ID not found in meta-data")

	return ""
}

type cloudInitInstanceData struct {
	V1 cloudInitV1 `json:"v1"`
}

type cloudInitV1 struct {
	CloudName  string `json:"cloud_name"`
	InstanceID string `json:"instance_id"`
	Zone       string `json:"availability_zone"`
}

func (c *client) readCloudInit(ctx context.Context, instanceFilePath string) (*cloudInitInstanceData, error) {
	logger := klog.FromContext(ctx)

	b, err := os.ReadFile(instanceFilePath)
	if err != nil {
		logger.Error(err, "Cannot read file "+instanceFilePath)

		return nil, err
	}

	var data cloudInitInstanceData
	if err := json.Unmarshal(b, &data); err != nil {
		logger.Error(err, "Cannot parse JSON file "+instanceFilePath)

		return nil, err
	}

	if strings.ToLower(data.V1.CloudName) != cloudStackCloudName {
		err := fmt.Errorf("cloud name from cloud-init is %s, only %s is supported", data.V1.CloudName, cloudStackCloudName)
		logger.Error(err, "Unsupported cloud name detected")

		return nil, err
	}

	return &data, nil
}

// readIgnition reads the ignition metadata file and returns the instance ID, or empty string if not found.
func (c *client) readIgnition(ctx context.Context, instanceFilePath string) (string, error) {
	logger := klog.FromContext(ctx)

	f, err := os.Open(instanceFilePath)
	if err != nil {
		logger.Error(err, "Cannot read file "+instanceFilePath)

		return "", err
	}
	defer f.Close()

	instanceID := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "COREOS_CLOUDSTACK_INSTANCE_ID") {
			lineSplit := strings.SplitAfter(line, "=")
			if len(lineSplit) == 2 {
				instanceID = strings.SplitAfter(line, "=")[1]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error(err, "Error scanning ignition metadata")
	}

	return instanceID, nil
}
