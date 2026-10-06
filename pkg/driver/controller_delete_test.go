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
	"errors"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud/fake"
)

// deleteTestConnector makes DeleteVolume/DeleteSnapshot fail with a given error
// and controls what the follow-up lookup finds (nil means "not found").
type deleteTestConnector struct {
	cloud.Interface

	deleteErr error
	volume    *cloud.Volume
	snapshot  *cloud.Snapshot
}

func (c *deleteTestConnector) DeleteVolume(context.Context, string) error { return c.deleteErr }

func (c *deleteTestConnector) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	if c.volume == nil {
		return nil, cloud.ErrNotFound
	}

	return c.volume, nil
}

func (c *deleteTestConnector) DeleteSnapshot(context.Context, string) error { return c.deleteErr }

func (c *deleteTestConnector) GetSnapshotByID(context.Context, string) (*cloud.Snapshot, error) {
	if c.snapshot == nil {
		return nil, cloud.ErrNotFound
	}

	return c.snapshot, nil
}

func TestDeleteVolumeVerifiesFailedDeletes(t *testing.T) {
	refused := errors.New("CloudStack API error 431 (CSExceptionErrorCode: 4350): Please specify a volume that is not attached to any VM")

	cases := []struct {
		name      string
		deleteErr error
		volume    *cloud.Volume
		want      codes.Code
	}{
		{"deleted", nil, nil, codes.OK},
		{"already not found", cloud.ErrNotFound, nil, codes.OK},
		{"error but volume gone", refused, nil, codes.OK},
		{"error and volume destroyed", refused, &cloud.Volume{ID: "v", State: "Destroy"}, codes.OK},
		{"error and volume attached", refused, &cloud.Volume{ID: "v", State: "Ready", VirtualMachineID: "vm-1"}, codes.FailedPrecondition},
		{"error and volume still there", refused, &cloud.Volume{ID: "v", State: "Ready"}, codes.Internal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cs := NewControllerServer(&deleteTestConnector{Interface: fake.New(), deleteErr: testCase.deleteErr, volume: testCase.volume})
			_, err := cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: "v"})
			if got := status.Code(err); got != testCase.want {
				t.Errorf("DeleteVolume() code = %v, want %v (err: %v)", got, testCase.want, err)
			}
		})
	}
}

func TestDeleteSnapshotVerifiesFailedDeletes(t *testing.T) {
	cases := []struct {
		name      string
		deleteErr error
		snapshot  *cloud.Snapshot
		want      codes.Code
	}{
		{"deleted", nil, nil, codes.OK},
		{"already not found", cloud.ErrNotFound, nil, codes.OK},
		{"unknown id", errors.New("CloudStack API error 431 (CSExceptionErrorCode: 4350): Invalid parameter id"), nil, codes.OK},
		{"already destroyed", errors.New(`{"errorcode":431,"errortext":"Snapshot [...] is already destroyed"}`), nil, codes.OK},
		{"removed record", errors.New(`{"errorcode":431,"errortext":"unable to find a snapshot with id 33"}`), nil, codes.OK},
		{"error and snapshot destroyed", errors.New("failed"), &cloud.Snapshot{ID: "s", State: "Destroyed"}, codes.OK},
		{"error and snapshot still there", errors.New("failed"), &cloud.Snapshot{ID: "s", State: "BackedUp"}, codes.Internal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cs := NewControllerServer(&deleteTestConnector{Interface: fake.New(), deleteErr: testCase.deleteErr, snapshot: testCase.snapshot})
			_, err := cs.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{SnapshotId: "s"})
			if got := status.Code(err); got != testCase.want {
				t.Errorf("DeleteSnapshot() code = %v, want %v (err: %v)", got, testCase.want, err)
			}
		})
	}
}
