package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"

	"github.com/shapeblue/cloudstack-csi-driver/pkg/cloud"
	"github.com/shapeblue/cloudstack-csi-driver/pkg/util"
)

// onlyVolumeCapAccessMode is the only volume capability access
// mode possible for CloudStack: SINGLE_NODE_WRITER, since a
// CloudStack volume can only be attached to a single node at
// any given time.
var onlyVolumeCapAccessMode = csi.VolumeCapability_AccessMode{
	Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
}

type controllerServer struct {
	csi.UnimplementedControllerServer
	// connector is the CloudStack client interface
	connector cloud.Interface

	// A map storing all volumes with ongoing operations so that additional operations
	// for that same volume (as defined by VolumeID/volume name) return an Aborted error
	volumeLocks *util.VolumeLocks

	// A map storing all volumes/snapshots with ongoing operations.
	operationLocks *util.OperationLock
}

// NewControllerServer creates a new Controller gRPC server.
func NewControllerServer(connector cloud.Interface) csi.ControllerServer {
	return &controllerServer{
		connector:      connector,
		volumeLocks:    util.NewVolumeLocks(),
		operationLocks: util.NewOperationLock(),
	}
}

//nolint:gocognit
func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("CreateVolume: called", "args", *req)

	// Check arguments.

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	name := req.GetName()

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}
	if !isValidVolumeCapabilities(volCaps) {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not supported. Only SINGLE_NODE_WRITER supported.")
	}

	if req.GetParameters() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume parameters missing in request")
	}
	diskOfferingID := req.GetParameters()[DiskOfferingKey]
	if diskOfferingID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Missing parameter %v", DiskOfferingKey)
	}

	if acquired := cs.volumeLocks.TryAcquire(name); !acquired {
		logger.Error(errors.New(util.ErrVolumeOperationAlreadyExistsVolumeName), "failed to acquire volume lock", "volumeName", name)

		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, name)
	}
	defer cs.volumeLocks.Release(name)

	// Check if a volume with that name already exists.
	vol, err := cs.connector.GetVolumeByName(ctx, name)
	if err != nil {
		if !errors.Is(err, cloud.ErrNotFound) {
			// Error with CloudStack
			return nil, status.Errorf(codes.Internal, "CloudStack error: %v", err)
		}
	} else {
		// The volume exists. Check if it suits the request.
		if ok, message := checkVolumeSuitable(vol, diskOfferingID, req.GetCapacityRange(), req.GetAccessibilityRequirements()); !ok {
			return nil, status.Errorf(codes.AlreadyExists, "Volume %v already exists but does not satisfy request: %s", name, message)
		}
		// Existing volume is ok.
		resp := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      vol.ID,
				CapacityBytes: vol.Size,
				VolumeContext: req.GetParameters(),
				// ContentSource: req.GetVolumeContentSource(), TODO: snapshot support.
				AccessibleTopology: []*csi.Topology{
					Topology{ZoneID: vol.ZoneID}.ToCSI(),
				},
			},
		}

		return resp, nil
	}

	// Check if this is a volume from snapshot
	var snapshotID string
	if src := req.GetVolumeContentSource(); src != nil {
		if snap := src.GetSnapshot(); snap != nil {
			snapshotID = snap.GetSnapshotId()
		}
	}

	// We have to create the volume.

	// Determine volume size using requested capacity range.
	sizeInGB, err := determineSize(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// If creating from snapshot, get the snapshot size
	var snapshotSizeGiB int64
	if snapshotID != "" {
		logger.Info("Creating volume from snapshot", "snapshotID", snapshotID)
		// Call the cloud connector's CreateVolumeFromSnapshot if implemented
		printVolumeAsJSON(req)
		snapshot, err := cs.connector.GetSnapshotByID(ctx, snapshotID)
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Snapshot %v not found", snapshotID)
		} else if err != nil {
			// Error with CloudStack
			return nil, status.Errorf(codes.Internal, "Error %v", err)
		}

		logger.Info("PVC created with", "size", sizeInGB)
		snapshotSizeGiB = util.RoundUpBytesToGB(snapshot.Size)
		if snapshotSizeGiB > sizeInGB {
			logger.Info("Snapshot size is greater than the request PVC, creating volume from snapshot of size", "snapshot size:", snapshotSizeGiB)
			sizeInGB = snapshotSizeGiB
		}

		volFromSnapshot, err := cs.connector.CreateVolumeFromSnapshot(ctx, snapshot.ZoneID, name, snapshot.ProjectID, snapshotID, sizeInGB)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Cannot create volume from snapshot %s: %v", snapshotID, err.Error())
		}

		resp := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      volFromSnapshot.ID,
				CapacityBytes: volFromSnapshot.Size,
				VolumeContext: req.GetParameters(),
				ContentSource: req.GetVolumeContentSource(),
				AccessibleTopology: []*csi.Topology{
					Topology{ZoneID: volFromSnapshot.ZoneID}.ToCSI(),
				},
			},
		}
		return resp, nil
	}

	// Determine zone using topology constraints.
	var zoneID string
	topologyRequirement := req.GetAccessibilityRequirements()
	if topologyRequirement == nil || topologyRequirement.GetRequisite() == nil { //nolint:nestif
		// No topology requirement. Use random zone.
		zones, err := cs.connector.ListZonesID(ctx)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		n := len(zones)
		if n == 0 {
			return nil, status.Error(codes.Internal, "No zone available")
		}
		zoneID = zones[rand.Intn(n)] //nolint:gosec
	} else {
		reqTopology := topologyRequirement.GetRequisite()
		if len(reqTopology) > 1 {
			return nil, status.Error(codes.InvalidArgument, "Too many topology requirements")
		}
		t, err := NewTopology(reqTopology[0])
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Cannot parse topology requirements")
		}
		zoneID = t.ZoneID
	}

	logger.Info("Creating new volume",
		"name", name,
		"size", sizeInGB,
		"offering", diskOfferingID,
		"zone", zoneID,
	)

	volID, err := cs.connector.CreateVolume(ctx, diskOfferingID, zoneID, name, sizeInGB)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot create volume %s: %v", name, err.Error())
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volID,
			CapacityBytes: util.GigaBytesToBytes(sizeInGB),
			VolumeContext: req.GetParameters(),
			ContentSource: req.GetVolumeContentSource(),
			AccessibleTopology: []*csi.Topology{
				Topology{ZoneID: zoneID}.ToCSI(),
			},
		},
	}

	return resp, nil
}

func printVolumeAsJSON(vol *csi.CreateVolumeRequest) {
	b, err := json.MarshalIndent(vol, "", "  ")
	if err != nil {
		klog.Errorf("Failed to marshal CreateVolumeRequest to JSON: %v", err)
		return
	}
	klog.V(5).Infof("CreateVolumeRequest as JSON:\n%s", string(b))
}

func checkVolumeSuitable(vol *cloud.Volume,
	diskOfferingID string, capRange *csi.CapacityRange, topologyRequirement *csi.TopologyRequirement,
) (bool, string) {
	if vol.DiskOfferingID != diskOfferingID {
		return false, fmt.Sprintf("Disk offering %s; requested disk offering %s", vol.DiskOfferingID, diskOfferingID)
	}

	if capRange != nil {
		if capRange.GetLimitBytes() > 0 && vol.Size > capRange.GetLimitBytes() {
			return false, fmt.Sprintf("Disk size %v bytes > requested limit size %v bytes", vol.Size, capRange.GetLimitBytes())
		}
		if capRange.GetRequiredBytes() > 0 && vol.Size < capRange.GetRequiredBytes() {
			return false, fmt.Sprintf("Disk size %v bytes < requested required size %v bytes", vol.Size, capRange.GetRequiredBytes())
		}
	}

	if topologyRequirement != nil && topologyRequirement.GetRequisite() != nil {
		reqTopology := topologyRequirement.GetRequisite()
		if len(reqTopology) > 1 {
			return false, "Too many topology requirements"
		}
		t, err := NewTopology(reqTopology[0])
		if err != nil {
			return false, "Cannot parse topology requirements"
		}
		if t.ZoneID != vol.ZoneID {
			return false, fmt.Sprintf("Volume in zone %s, requested zone is %s", vol.ZoneID, t.ZoneID)
		}
	}

	return true, ""
}

func determineSize(req *csi.CreateVolumeRequest) (int64, error) {
	var sizeInGB int64

	if req.GetCapacityRange() != nil {
		capRange := req.GetCapacityRange()

		required := capRange.GetRequiredBytes()
		sizeInGB = util.RoundUpBytesToGB(required)
		if sizeInGB == 0 {
			sizeInGB = 1
		}

		if limit := capRange.GetLimitBytes(); limit > 0 {
			if util.GigaBytesToBytes(sizeInGB) > limit {
				return 0, fmt.Errorf("after round-up, volume size %v GB exceeds the limit specified of %v bytes", sizeInGB, limit)
			}
		}
	}

	if sizeInGB == 0 {
		sizeInGB = 1
	}

	return sizeInGB, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("DeleteVolume: called", "args", *req)

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	volumeID := req.GetVolumeId()

	if acquired := cs.volumeLocks.TryAcquire(volumeID); !acquired {
		logger.Error(errors.New(util.ErrVolumeOperationAlreadyExistsVolumeID), "failed to acquire volume lock", "volumeID", volumeID)

		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer cs.volumeLocks.Release(volumeID)

	// lock out volumeID for clone and expand operation
	if err := cs.operationLocks.GetDeleteLock(volumeID); err != nil {
		logger.Error(err, "Failed to acquire delete operation lock")

		return nil, status.Error(codes.Aborted, err.Error())
	}
	defer cs.operationLocks.ReleaseDeleteLock(volumeID)

	logger.Info("Deleting volume",
		"volumeID", volumeID,
	)

	err := cs.connector.DeleteVolume(ctx, volumeID)
	if err != nil && !errors.Is(err, cloud.ErrNotFound) {
		return nil, status.Errorf(codes.Internal, "Cannot delete volume %s: %s", volumeID, err.Error())
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).Infof("CreateSnapshot")

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot name missing in request")
	}

	volumeID := req.GetSourceVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "SourceVolumeId missing in request")
	}

	volume, err := cs.connector.GetVolumeByID(ctx, volumeID)
	if err != nil {
		if err.Error() == "invalid volume ID: empty string" {
			return nil, status.Error(codes.InvalidArgument, "Invalid volume ID")
		}
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Volume %v not found", volumeID)
		}
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}
	klog.V(4).Infof("CreateSnapshot of volume: %s", volume.ID)
	snapshot, err := cs.connector.CreateSnapshot(ctx, volume.ID, req.GetName())
	if errors.Is(err, cloud.ErrAlreadyExists) {
		return nil, status.Errorf(codes.AlreadyExists, "Snapshot name conflict: already exists for a different source volume")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create snapshot for volume %s: %v", volume.ID, err.Error())
	}

	t, err := time.Parse("2006-01-02T15:04:05-0700", snapshot.CreatedAt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to parse snapshot creation time: %v", err)
	}

	ts := timestamppb.New(t)

	resp := &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.ID,
			SourceVolumeId: volume.ID,
			CreationTime:   ts,
			ReadyToUse:     true,
		},
	}
	return resp, nil
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	entries := []*csi.ListSnapshotsResponse_Entry{}

	snapshots, err := cs.connector.ListSnapshots(ctx, req.GetSourceVolumeId(), req.GetSnapshotId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list snapshots: %v", err)
	}

	// Pagination logic
	start := 0
	if req.StartingToken != "" {
		var err error
		start, err = strconv.Atoi(req.StartingToken)
		if err != nil || start < 0 || start > len(snapshots) {
			return nil, status.Error(codes.Aborted, "Invalid startingToken")
		}
	}
	maxEntries := int(req.MaxEntries)
	end := len(snapshots)
	if maxEntries > 0 && start+maxEntries < end {
		end = start + maxEntries
	}
	nextToken := ""
	if end < len(snapshots) {
		nextToken = strconv.Itoa(end)
	}

	for i := start; i < end; i++ {
		snap := snapshots[i]
		t, _ := time.Parse("2006-01-02T15:04:05-0700", snap.CreatedAt)
		ts := timestamppb.New(t)
		entry := &csi.ListSnapshotsResponse_Entry{
			Snapshot: &csi.Snapshot{
				SnapshotId:     snap.ID,
				SourceVolumeId: snap.VolumeID,
				CreationTime:   ts,
				ReadyToUse:     true,
			},
		}
		entries = append(entries, entry)
	}
	return &csi.ListSnapshotsResponse{Entries: entries, NextToken: nextToken}, nil
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	snapshotID := req.GetSnapshotId()

	if snapshotID == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID missing in request")
	}

	klog.V(4).Infof("DeleteSnapshot for snapshotID: %s", snapshotID)

	err := cs.connector.DeleteSnapshot(ctx, snapshotID)
	if errors.Is(err, cloud.ErrNotFound) {
		// Per CSI spec, return OK if snapshot does not exist
		return &csi.DeleteSnapshotResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("ControllerPublishVolume: called", "args", *req)

	// Check arguments.

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	volumeID := req.GetVolumeId()

	if req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Node ID missing in request")
	}
	nodeID := req.GetNodeId()

	if req.GetReadonly() {
		return nil, status.Error(codes.InvalidArgument, "Readonly not possible")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if req.GetVolumeCapability().GetAccessMode().GetMode() != onlyVolumeCapAccessMode.GetMode() {
		return nil, status.Error(codes.InvalidArgument, "Access mode not accepted")
	}

	logger.Info("Initiating attaching volume",
		"volumeID", volumeID,
		"nodeID", nodeID,
	)

	// Check volume.
	vol, err := cs.connector.GetVolumeByID(ctx, volumeID)
	if errors.Is(err, cloud.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "Volume %v not found", volumeID)
	} else if err != nil {
		// Error with CloudStack
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	if vol.VirtualMachineID != "" && vol.VirtualMachineID != nodeID {
		logger.Error(nil, "Volume already attached to another node",
			"volumeID", volumeID,
			"nodeID", nodeID,
			"attachedNodeID", vol.VirtualMachineID,
		)

		return nil, status.Error(codes.AlreadyExists, "Volume already assigned to another node")
	}

	if _, err := cs.connector.GetVMByID(ctx, nodeID); errors.Is(err, cloud.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "VM %v not found", nodeID)
	} else if err != nil {
		// Error with CloudStack
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	if vol.VirtualMachineID == nodeID {
		// volume already attached.
		logger.Info("Volume already attached to node",
			"volumeID", volumeID,
			"nodeID", nodeID,
			"deviceID", vol.DeviceID,
		)
		publishContext := map[string]string{
			deviceIDContextKey: vol.DeviceID,
		}

		return &csi.ControllerPublishVolumeResponse{PublishContext: publishContext}, nil
	}

	logger.Info("Attaching volume to node",
		"volumeID", volumeID,
		"nodeID", nodeID,
	)

	deviceID, err := cs.connector.AttachVolume(ctx, volumeID, nodeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot attach volume %s: %s", volumeID, err.Error())
	}

	logger.Info("Attached volume to node successfully",
		"volumeID", volumeID,
		"nodeID", nodeID,
	)

	publishContext := map[string]string{
		deviceIDContextKey: deviceID,
	}

	return &csi.ControllerPublishVolumeResponse{PublishContext: publishContext}, nil
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("ControllerUnpublishVolume: called", "args", *req)

	// Check arguments.

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	volumeID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	// Check volume.
	if vol, err := cs.connector.GetVolumeByID(ctx, volumeID); errors.Is(err, cloud.ErrNotFound) {
		// Volume does not exist in CloudStack. We can safely assume this volume is no longer attached
		// The spec requires us to return OK here.
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	} else if err != nil {
		// Error with CloudStack
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	} else if nodeID != "" && vol.VirtualMachineID != nodeID {
		// Volume is present but not attached to this particular nodeID
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	// Check VM existence.
	if _, err := cs.connector.GetVMByID(ctx, nodeID); errors.Is(err, cloud.ErrNotFound) {
		// volumes cannot be attached to deleted VMs.
		logger.Error(nil, "VM not found, marking ControllerUnpublishVolume successful",
			"volumeID", volumeID,
			"nodeID", nodeID,
		)

		return &csi.ControllerUnpublishVolumeResponse{}, nil
	} else if err != nil {
		// Error with CloudStack
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	logger.Info("Detaching volume from node",
		"volumeID", volumeID,
		"nodeID", nodeID,
	)

	err := cs.connector.DetachVolume(ctx, volumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot detach volume %s: %s", volumeID, err.Error())
	}

	logger.Info("Detached volume from node successfully",
		"volumeID", volumeID,
		"nodeID", nodeID,
	)

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("ValidateVolumeCapabilities: called", "args", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if _, err := cs.connector.GetVolumeByID(ctx, volumeID); errors.Is(err, cloud.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "Volume %v not found", volumeID)
	} else if err != nil {
		// Error with CloudStack
		return nil, status.Errorf(codes.Internal, "Error %v", err)
	}

	if !isValidVolumeCapabilities(volCaps) {
		return &csi.ValidateVolumeCapabilitiesResponse{Message: "Requested VolumeCapabilities are invalid"}, nil
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: volCaps,
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	for _, c := range volCaps {
		if c.GetAccessMode() != nil && c.GetAccessMode().GetMode() != onlyVolumeCapAccessMode.GetMode() {
			return false
		}
	}

	return true
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("ControllerExpandVolume: called", "args", protosanitizer.StripSecrets(*req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}

	// lock out parallel requests against the same volume ID
	if acquired := cs.volumeLocks.TryAcquire(volumeID); !acquired {
		logger.Error(errors.New(util.ErrVolumeOperationAlreadyExistsVolumeID), "failed to acquire volume lock", "volumeID", volumeID)

		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer cs.volumeLocks.Release(volumeID)

	volSizeBytes := capRange.GetRequiredBytes()
	volSizeGB := util.RoundUpBytesToGB(volSizeBytes)
	maxVolSize := capRange.GetLimitBytes()

	if maxVolSize > 0 && maxVolSize < util.GigaBytesToBytes(volSizeGB) {
		return nil, status.Error(codes.OutOfRange, "Volume size exceeds the limit specified")
	}

	_, err := cs.connector.GetVolumeByID(ctx, volumeID)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Volume %v not found", volumeID)
		}

		return nil, status.Error(codes.Internal, fmt.Sprintf("GetVolume failed with error %v", err))
	}

	// lock out volumeID for clone and delete operation
	if err := cs.operationLocks.GetExpandLock(volumeID); err != nil {
		logger.Error(err, "failed acquiring expand lock", "volumeID", volumeID)

		return nil, status.Error(codes.Aborted, err.Error())
	}
	defer cs.operationLocks.ReleaseExpandLock(volumeID)

	err = cs.connector.ExpandVolume(ctx, volumeID, volSizeGB)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not resize volume %q to size %v: %v", volumeID, volSizeGB, err)
	}

	logger.Info("Volume successfully expanded",
		"volumeID", volumeID,
		"volumeSize", volSizeGB,
	)

	nodeExpansionRequired := true
	// Node expansion is not required for raw block volumes.
	volCap := req.GetVolumeCapability()
	if volCap != nil && volCap.GetBlock() != nil {
		nodeExpansionRequired = false
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         util.GigaBytesToBytes(volSizeGB),
		NodeExpansionRequired: nodeExpansionRequired,
	}, nil
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	logger := klog.FromContext(ctx)
	logger.V(6).Info("ControllerGetCapabilities: called", "args", protosanitizer.StripSecrets(*req))

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
					},
				},
			},
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
					},
				},
			},
		},
	}

	return resp, nil
}
