package cloud

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *client) GetSnapshotByID(ctx context.Context, snapshotID ...string) (*Snapshot, error) {
	p := c.Snapshot.NewListSnapshotsParams()
	if snapshotID != nil {
		p.SetId(snapshotID[0])
	}
	l, err := c.Snapshot.ListSnapshots(p)
	if err != nil {
		return nil, err
	}
	if l.Count == 0 {
		return nil, ErrNotFound
	}
	if l.Count > 1 {
		return nil, ErrTooManyResults
	}
	snapshot := l.Snapshots[0]
	s := Snapshot{
		ID:        snapshot.Id,
		Name:      snapshot.Name,
		DomainID:  snapshot.Domainid,
		ProjectID: snapshot.Projectid,
		ZoneID:    snapshot.Zoneid,
		VolumeID:  snapshot.Volumeid,
	}

	return &s, nil
}

func (c *client) CreateSnapshot(ctx context.Context, volumeID string) (*Snapshot, error) {
	p := c.Snapshot.NewCreateSnapshotParams(volumeID)
	snapshot, err := c.Snapshot.CreateSnapshot(p)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	snap := Snapshot{
		ID:        snapshot.Id,
		Name:      snapshot.Name,
		DomainID:  snapshot.Domainid,
		ProjectID: snapshot.Projectid,
		ZoneID:    snapshot.Zoneid,
		VolumeID:  snapshot.Volumeid,
		CreatedAt: snapshot.Created,
	}
	return &snap, nil
}

func (c *client) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	p := c.Snapshot.NewDeleteSnapshotParams(snapshotID)
	_, err := c.Snapshot.DeleteSnapshot(p)
	if err != nil && strings.Contains(err.Error(), "4350") {
		// CloudStack error InvalidParameterValueException
		return ErrNotFound
	}

	return err
}
