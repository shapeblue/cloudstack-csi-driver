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

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud/fake"
)

func TestCreateSnapshotIsIdempotent(t *testing.T) {
	cs := NewControllerServer(fake.New())
	req := &csi.CreateSnapshotRequest{
		Name:           "snap-idem",
		SourceVolumeId: "ace9f28b-3081-40c1-8353-4cc3e3014072",
	}

	first, err := cs.CreateSnapshot(context.Background(), req)
	if err != nil {
		t.Fatalf("first CreateSnapshot failed: %v", err)
	}
	second, err := cs.CreateSnapshot(context.Background(), req)
	if err != nil {
		t.Fatalf("second CreateSnapshot failed: %v", err)
	}
	if first.GetSnapshot().GetSnapshotId() != second.GetSnapshot().GetSnapshotId() {
		t.Errorf("expected same snapshot ID on retry, got %q and %q",
			first.GetSnapshot().GetSnapshotId(), second.GetSnapshot().GetSnapshotId())
	}
}
