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

// DriverName is the name of the CSI plugin.
const DriverName = "csi.cloudstack.apache.org"

// Mode is the operating mode of the CSI driver.
type Mode string

// Driver operating modes.
const (
	// ControllerMode is the mode that only starts the controller service.
	ControllerMode Mode = "controller"
	// NodeMode is the mode that only starts the node service.
	NodeMode Mode = "node"
	// AllMode is the mode that only starts both the controller and the node service.
	AllMode Mode = "all"
)

// constants for default command line flag values.
const (
	// DefaultCSIEndpoint is the default CSI endpoint for the driver.
	DefaultCSIEndpoint             = "unix://tmp/csi.sock"
	DefaultMaxVolAttachLimit int64 = 256
)

// Filesystem types.
const (
	// FSTypeExt2 represents the ext2 filesystem type.
	FSTypeExt2 = "ext2"
	// FSTypeExt3 represents the ext3 filesystem type.
	FSTypeExt3 = "ext3"
	// FSTypeExt4 represents the ext4 filesystem type.
	FSTypeExt4 = "ext4"
	// FSTypeXfs represents the xfs filesystem type.
	FSTypeXfs = "xfs"
)

// Topology keys.
const (
	ZoneKey = "topology." + DriverName + "/zone"
	HostKey = "topology." + DriverName + "/host"
)

// Volume parameters keys.
const (
	DiskOfferingKey = DriverName + "/disk-offering-id"
)

const deviceIDContextKey = "deviceID"
