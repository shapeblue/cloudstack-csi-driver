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
	"context"

	"k8s.io/klog/v2"
)

func (c *client) GetNodeInfo(ctx context.Context, vmName string) (*VM, error) {
	logger := klog.FromContext(ctx)

	// First, try to read the instance ID from meta-data.
	if id := c.metadataInstanceID(ctx); id != "" {
		// Instance ID found using metadata
		logger.V(4).Info("Looking up node info using VM ID found in metadata", "nodeID", id)

		// Use CloudStack API to get VM info
		return c.GetVMByID(ctx, id)
	}

	// VM ID was not found using metadata, fall back to using VM name instead.
	logger.V(4).Info("Looking up node info using VM name", "nodeName", vmName)

	return c.getVMByName(ctx, vmName)
}
