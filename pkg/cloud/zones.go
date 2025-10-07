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

func (c *client) ListZonesID(ctx context.Context) ([]string, error) {
	logger := klog.FromContext(ctx)
	result := make([]string, 0)
	p := c.Zone.NewListZonesParams()
	p.SetAvailable(true)
	logger.V(2).Info("CloudStack API call", "command", "ListZones", "params", map[string]string{
		"available": "true",
	})
	r, err := c.Zone.ListZones(p)
	if err != nil {
		return result, err
	}
	for _, zone := range r.Zones {
		result = append(result, zone.Id)
	}

	return result, nil
}
