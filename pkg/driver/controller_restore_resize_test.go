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
	"context"
	"testing"

	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud/fake"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/util"
)

func TestGrowVolumeFromSnapshot(t *testing.T) {
	cs, ok := NewControllerServer(fake.New()).(*controllerServer)
	if !ok {
		t.Fatal("unexpected controller server type")
	}

	// Volume known to the fake connector, smaller than requested.
	vol := &cloud.Volume{ID: "ace9f28b-3081-40c1-8353-4cc3e3014072", Size: 10}
	if err := cs.growVolumeFromSnapshot(context.Background(), vol, "snap-1", 5); err != nil {
		t.Fatalf("growVolumeFromSnapshot failed: %v", err)
	}
	if want := util.GigaBytesToBytes(5); vol.Size != want {
		t.Errorf("expected size %d, got %d", want, vol.Size)
	}

	// Already large enough: no-op.
	big := &cloud.Volume{ID: "does-not-matter", Size: util.GigaBytesToBytes(10)}
	if err := cs.growVolumeFromSnapshot(context.Background(), big, "snap-1", 5); err != nil {
		t.Errorf("expected no-op for large enough volume, got %v", err)
	}

	// Resize failure (unknown volume) returns an error.
	missing := &cloud.Volume{ID: "unknown", Size: 10}
	if err := cs.growVolumeFromSnapshot(context.Background(), missing, "snap-1", 5); err == nil {
		t.Error("expected error when resize fails")
	}
}
