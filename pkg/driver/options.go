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

package driver

import (
	"errors"

	flag "github.com/spf13/pflag"
)

// Options contains options and configuration settings for the driver.
type Options struct {
	Mode Mode

	// #### Server options ####

	// Endpoint is the endpoint for the CSI driver server
	Endpoint string

	// CloudStackConfig is the path to the CloudStack configuration file
	CloudStackConfig string

	// #### Node options #####

	// NodeName is used to retrieve the node instance ID in case metadata lookup fails.
	NodeName string

	// VolumeAttachLimit specifies the value that shall be reported as "maximum number of attachable volumes"
	// in CSINode objects. It is similar to https://kubernetes.io/docs/concepts/storage/storage-limits/#custom-limits
	// which allowed administrators to specify custom volume limits by configuring the kube-scheduler.
	VolumeAttachLimit int64
}

func (o *Options) AddFlags(f *flag.FlagSet) {
	// Server options
	f.StringVar(&o.Endpoint, "endpoint", DefaultCSIEndpoint, "Endpoint for the CSI driver server")
	f.StringVar(&o.CloudStackConfig, "cloudstack-config", "./cloud-config", "Path to CloudStack configuration file")

	// Node options
	if o.Mode == AllMode || o.Mode == NodeMode {
		f.StringVar(&o.NodeName, "node-name", "", "Node name used to look up instance ID in case metadata lookup fails")
		f.Int64Var(&o.VolumeAttachLimit, "volume-attach-limit", DefaultMaxVolAttachLimit, "Value for the maximum number of volumes attachable per node.")
	}
}

func (o *Options) Validate() error {
	if o.Mode == AllMode || o.Mode == NodeMode {
		if o.VolumeAttachLimit < 1 || o.VolumeAttachLimit > 256 {
			return errors.New("invalid --volume-attach-limit specified, allowed range is 1 to 256")
		}
	}

	return nil
}
