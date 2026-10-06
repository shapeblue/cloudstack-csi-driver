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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud/fake"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/util"
)

const restoreTestSourceVolume = "ace9f28b-3081-40c1-8353-4cc3e3014072"

// maxSizeConnector reports a maximum custom disk size and counts restores.
type maxSizeConnector struct {
	cloud.Interface

	maxGB    int64
	restores int
}

func (c *maxSizeConnector) GetMaxCustomDiskSizeGB(context.Context) (int64, error) {
	return c.maxGB, nil
}

func (c *maxSizeConnector) CreateVolumeFromSnapshot(ctx context.Context, zoneID, name, projectID, snapshotID string, sizeInGB int64) (*cloud.Volume, error) {
	c.restores++

	return c.Interface.CreateVolumeFromSnapshot(ctx, zoneID, name, projectID, snapshotID, sizeInGB)
}

func restoreRequest(t *testing.T, cs csi.ControllerServer, name string, sizeGB int64) *csi.CreateVolumeRequest {
	t.Helper()

	snap, err := cs.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		Name:           "snap-for-" + name,
		SourceVolumeId: restoreTestSourceVolume,
	})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	return &csi.CreateVolumeRequest{
		Name:          name,
		CapacityRange: &csi.CapacityRange{RequiredBytes: util.GigaBytesToBytes(sizeGB)},
		VolumeCapabilities: []*csi.VolumeCapability{{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
		}},
		// Differs from the disk offering the restored volume gets.
		Parameters: map[string]string{DiskOfferingKey: "storageclass-offering"},
		VolumeContentSource: &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: snap.GetSnapshot().GetSnapshotId()},
			},
		},
	}
}

func TestCreateVolumeRetryReusesRestoredVolume(t *testing.T) {
	cs := NewControllerServer(fake.New())
	req := restoreRequest(t, cs, "pvc-retry", 1)

	first, err := cs.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("first CreateVolume failed: %v", err)
	}

	// Retry with the same name, as the provisioner does after a timeout.
	second, err := cs.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("retried CreateVolume failed: %v", err)
	}
	if second.GetVolume().GetVolumeId() != first.GetVolume().GetVolumeId() {
		t.Errorf("retry returned volume %q, want the existing %q", second.GetVolume().GetVolumeId(), first.GetVolume().GetVolumeId())
	}
	if second.GetVolume().GetContentSource() == nil {
		t.Error("retry did not return the content source; the provisioner would delete the volume")
	}
}

func TestCreateVolumeRetryGrowsUndersizedRestoredVolume(t *testing.T) {
	cs := NewControllerServer(fake.New())
	req := restoreRequest(t, cs, "pvc-grow", 1)
	if _, err := cs.CreateVolume(context.Background(), req); err != nil {
		t.Fatalf("first CreateVolume failed: %v", err)
	}

	// The retry asks for more than the earlier attempt created.
	req.CapacityRange = &csi.CapacityRange{RequiredBytes: util.GigaBytesToBytes(3)}
	resp, err := cs.CreateVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("retried CreateVolume failed: %v", err)
	}
	if got, want := resp.GetVolume().GetCapacityBytes(), util.GigaBytesToBytes(3); got != want {
		t.Errorf("capacity = %d, want %d", got, want)
	}
}

func TestCreateVolumeFromSnapshotAboveMaxSizeFailsBeforeRestoring(t *testing.T) {
	conn := &maxSizeConnector{Interface: fake.New(), maxGB: 1024}
	cs := NewControllerServer(conn)
	req := restoreRequest(t, cs, "pvc-huge", 2000)

	_, err := cs.CreateVolume(context.Background(), req)
	if got := status.Code(err); got != codes.OutOfRange {
		t.Errorf("CreateVolume code = %v, want OutOfRange (err: %v)", got, err)
	}
	if conn.restores != 0 {
		t.Errorf("snapshot was restored %d times; want 0 when the size is above the maximum", conn.restores)
	}
}
