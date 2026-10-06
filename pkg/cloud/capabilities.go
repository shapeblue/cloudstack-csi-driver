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

func (c *client) GetMaxCustomDiskSizeGB(ctx context.Context) (int64, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("CloudStack API call", "command", "ListCapabilities")

	resp, err := c.Configuration.ListCapabilities(c.Configuration.NewListCapabilitiesParams())
	if err != nil {
		return 0, err
	}
	if resp.Capabilities == nil {
		return 0, nil
	}

	return resp.Capabilities.Customdiskofferingmaxsize, nil
}
