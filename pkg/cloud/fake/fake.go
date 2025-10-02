// Package fake provides a fake implementation of the cloud
// connector interface, to be used in tests.
package fake

import (
	"context"
	"errors"

	"github.com/hashicorp/go-uuid"

	"github.com/shapeblue/cloudstack-csi-driver/pkg/cloud"
	"github.com/shapeblue/cloudstack-csi-driver/pkg/util"
)

const zoneID = "a1887604-237c-4212-a9cd-94620b7880fa"

type fakeConnector struct {
	node            *cloud.VM
	snapshot        *cloud.Snapshot
	volumesByID     map[string]cloud.Volume
	volumesByName   map[string]cloud.Volume
	snapshotsByName map[string]*cloud.Snapshot
}

// New returns a new fake implementation of the
// CloudStack connector.
func New() cloud.Interface {
	volume := cloud.Volume{
		ID:               "ace9f28b-3081-40c1-8353-4cc3e3014072",
		Name:             "vol-1",
		Size:             10,
		DiskOfferingID:   "9743fd77-0f5d-4ef9-b2f8-f194235c769c",
		ZoneID:           zoneID,
		VirtualMachineID: "",
		DeviceID:         "",
	}
	node := &cloud.VM{
		ID:     "0d7107a3-94d2-44e7-89b8-8930881309a5",
		ZoneID: zoneID,
	}

	snapshot := &cloud.Snapshot{
		ID:        "9d076136-657b-4c84-b279-455da3ea484c",
		Name:      "pvc-vol-snap-1",
		DomainID:  "51f0fcb5-db16-4637-94f5-30131010214f",
		ZoneID:    zoneID,
		VolumeID:  "4f1f610d-6f17-4ff9-9228-e4062af93e54",
		CreatedAt: "2025-07-07T16:13:06-0700",
	}

	snapshotsByName := map[string]*cloud.Snapshot{
		snapshot.Name: snapshot,
	}

	return &fakeConnector{
		node:            node,
		snapshot:        snapshot,
		volumesByID:     map[string]cloud.Volume{volume.ID: volume},
		volumesByName:   map[string]cloud.Volume{volume.Name: volume},
		snapshotsByName: snapshotsByName,
	}
}

func (f *fakeConnector) GetVMByID(_ context.Context, vmID string) (*cloud.VM, error) {
	if vmID == f.node.ID {
		return f.node, nil
	}

	return nil, cloud.ErrNotFound
}

func (f *fakeConnector) GetNodeInfo(_ context.Context, _ string) (*cloud.VM, error) {
	return f.node, nil
}

func (f *fakeConnector) ListZonesID(_ context.Context) ([]string, error) {
	return []string{zoneID}, nil
}

func (f *fakeConnector) GetVolumeByID(_ context.Context, volumeID string) (*cloud.Volume, error) {
	if volumeID == "" {
		return nil, errors.New("invalid volume ID: empty string")
	}
	vol, ok := f.volumesByID[volumeID]
	if ok {
		return &vol, nil
	}

	return nil, cloud.ErrNotFound
}

func (f *fakeConnector) GetVolumeByName(_ context.Context, name string) (*cloud.Volume, error) {
	if name == "" {
		return nil, errors.New("invalid volume name: empty string")
	}
	vol, ok := f.volumesByName[name]
	if ok {
		return &vol, nil
	}

	return nil, cloud.ErrNotFound
}

func (f *fakeConnector) CreateVolume(_ context.Context, diskOfferingID, zoneID, name string, sizeInGB int64) (string, error) {
	id, _ := uuid.GenerateUUID()
	vol := cloud.Volume{
		ID:             id,
		Name:           name,
		Size:           util.GigaBytesToBytes(sizeInGB),
		DiskOfferingID: diskOfferingID,
		ZoneID:         zoneID,
	}
	f.volumesByID[vol.ID] = vol
	f.volumesByName[vol.Name] = vol

	return vol.ID, nil
}

func (f *fakeConnector) DeleteVolume(_ context.Context, id string) error {
	if vol, ok := f.volumesByID[id]; ok {
		name := vol.Name
		delete(f.volumesByName, name)
	}
	delete(f.volumesByID, id)

	return nil
}

func (f *fakeConnector) AttachVolume(_ context.Context, _, _ string) (string, error) {
	return "1", nil
}

func (f *fakeConnector) DetachVolume(_ context.Context, _ string) error {
	return nil
}

func (f *fakeConnector) ExpandVolume(_ context.Context, volumeID string, newSizeInGB int64) error {
	if vol, ok := f.volumesByID[volumeID]; ok {
		newSizeInBytes := newSizeInGB * 1024 * 1024 * 1024
		if newSizeInBytes > vol.Size {
			vol.Size = newSizeInBytes
			f.volumesByID[volumeID] = vol
			f.volumesByName[vol.Name] = vol
		}

		return nil
	}

	return cloud.ErrNotFound
}

func (f *fakeConnector) CreateVolumeFromSnapshot(ctx context.Context, zoneID, name, projectID, snapshotID string, sizeInGB int64) (*cloud.Volume, error) {
	vol := &cloud.Volume{
		ID:             "fake-vol-from-snap-" + name,
		Name:           name,
		Size:           util.GigaBytesToBytes(sizeInGB),
		DiskOfferingID: "fake-disk-offering",
		ZoneID:         zoneID,
	}
	f.volumesByID[vol.ID] = *vol
	f.volumesByName[vol.Name] = *vol
	return vol, nil
}

func (f *fakeConnector) GetSnapshotByID(_ context.Context, snapshotID string) (*cloud.Snapshot, error) {
	return f.snapshot, nil
}

func (f *fakeConnector) CreateSnapshot(_ context.Context, volumeID string) (*cloud.Snapshot, error) {
	return f.snapshot, nil
}

func (f *fakeConnector) DeleteSnapshot(_ context.Context, snapshotID string) error {
	return nil
}

func (f *fakeConnector) GetSnapshotByName(_ context.Context, name string) (*cloud.Snapshot, error) {
	if name == "" {
		return nil, errors.New("invalid snapshot name: empty string")
	}
	if snap, ok := f.snapshotsByName[name]; ok {
		return snap, nil
	}
	return nil, cloud.ErrNotFound
}
