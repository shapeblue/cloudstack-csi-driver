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

	"github.com/container-storage-interface/spec/lib/go/csi"
)

// Topology represents CloudStack storage topology.
type Topology struct {
	ZoneID string
	HostID string
}

// NewTopology converts a *csi.Topology to Topology.
func NewTopology(t *csi.Topology) (Topology, error) {
	segments := t.GetSegments()
	if segments == nil {
		return Topology{}, errors.New("nil segment in topology")
	}

	zoneID, ok := segments[ZoneKey]
	if !ok {
		return Topology{}, errors.New("no zone in topology")
	}
	hostID := segments[HostKey]

	return Topology{zoneID, hostID}, nil
}

// ToCSI converts a Topology to a *csi.Topology.
func (t Topology) ToCSI() *csi.Topology {
	segments := make(map[string]string)
	segments[ZoneKey] = t.ZoneID
	if t.HostID != "" {
		segments[HostKey] = t.HostID
	}

	return &csi.Topology{
		Segments: segments,
	}
}
