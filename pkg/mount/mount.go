// Package mount provides utilities to detect,
// format and mount storage devices.
package mount

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	kexec "k8s.io/utils/exec"
)

const (
	diskIDPath = "/dev/disk/by-id"
)

// Interface defines the set of methods to allow for
// mount operations on a system.
type Interface interface { //nolint:interfacebloat
	mount.Interface

	FormatAndMount(source string, target string, fstype string, options []string) error
	GetBlockSizeBytes(devicePath string) (int64, error)
	GetDevicePath(ctx context.Context, volumeID string) (string, error)
	GetDeviceName(mountPath string) (string, int, error)
	GetStatistics(volumePath string) (volumeStatistics, error)
	IsBlockDevice(devicePath string) (bool, error)
	IsCorruptedMnt(err error) bool
	MakeDir(pathname string) error
	MakeFile(pathname string) error
	NeedResize(devicePath string, deviceMountPath string) (bool, error)
	PathExists(path string) (bool, error)
	Resize(devicePath, deviceMountPath string) (bool, error)
	Unpublish(path string) error
	Unstage(path string) error
}

type mounter struct {
	*mount.SafeFormatAndMount
}

type volumeStatistics struct {
	AvailableBytes, TotalBytes, UsedBytes    int64
	AvailableInodes, TotalInodes, UsedInodes int64
}

// New creates an implementation of the mount.Interface.
func New() Interface {
	return &mounter{
		&mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      kexec.New(),
		},
	}
}

// GetBlockSizeBytes gets the size of the disk in bytes.
func (m *mounter) GetBlockSizeBytes(devicePath string) (int64, error) {
	output, err := m.Exec.Command("blockdev", "--getsize64", devicePath).Output()
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %w", devicePath, string(output), err)
	}
	strOut := strings.TrimSpace(string(output))
	gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse size %s as int", strOut)
	}

	return gotSizeBytes, nil
}

func (m *mounter) GetDevicePath(ctx context.Context, volumeID string) (string, error) {
	logger := klog.FromContext(ctx)
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   1.5,
		Steps:    20,
	}

	var devicePath string
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(context.Context) (bool, error) {
		path, err := m.getDevicePathBySerialID(ctx, volumeID)
		if err != nil {
			return false, err
		}
		if path != "" {
			devicePath = path
			logger.V(4).Info("Device path found", "volumeID", volumeID, "devicePath", path)
			return true, nil
		}
		m.probeVolume(ctx)

		return false, nil
	})

	if wait.Interrupted(err) {
		return "", fmt.Errorf("failed to find device for the volumeID: %q within the alloted time", volumeID)
	} else if devicePath == "" {
		return "", fmt.Errorf("device path was empty for volumeID: %q", volumeID)
	}

	return devicePath, nil
}

func (m *mounter) getDevicePathBySerialID(ctx context.Context, volumeID string) (string, error) {
	logger := klog.FromContext(ctx)

	// First try XenServer device paths
	xenDevicePath, err := m.getDevicePathForXenServer(ctx, volumeID)
	if err != nil {
		logger.V(4).Info("Failed to get XenServer device path", "volumeID", volumeID, "error", err)
	}
	if xenDevicePath != "" {
		return xenDevicePath, nil
	}

	// Try VMware device paths
	vmwareDevicePath, err := m.getDevicePathForVMware(ctx, volumeID)
	if err != nil {
		logger.V(4).Info("Failed to get VMware device path", "volumeID", volumeID, "error", err)
	}
	if vmwareDevicePath != "" {
		return vmwareDevicePath, nil
	}
	// Fall back to standard device paths (for KVM)
	sourcePathPrefixes := []string{"virtio-", "scsi-", "scsi-0QEMU_QEMU_HARDDISK_"}
	serial := diskUUIDToSerial(volumeID)
	for _, prefix := range sourcePathPrefixes {
		source := filepath.Join(diskIDPath, prefix+serial)
		_, err := os.Stat(source)
		if err == nil {
			return source, nil
		}
		if !os.IsNotExist(err) {
			logger.Error(err, "Failed to stat device path", "path", source)
			return "", err
		}
	}

	return "", nil
}

func (m *mounter) getDevicePathForXenServer(ctx context.Context, volumeID string) (string, error) {
	logger := klog.FromContext(ctx)

	for i := 'b'; i <= 'z'; i++ {
		devicePath := fmt.Sprintf("/dev/xvd%c", i)
		logger.V(5).Info("Checking XenServer device path", "devicePath", devicePath, "volumeID", volumeID)

		if _, err := os.Stat(devicePath); err == nil {
			isBlock, err := m.IsBlockDevice(devicePath)
			if err == nil && isBlock {
				if m.verifyDevice(ctx, devicePath, volumeID) {
					logger.V(4).Info("Found and verified XenServer device", "devicePath", devicePath, "volumeID", volumeID)
					return devicePath, nil
				}
			}
		}
	}
	return "", fmt.Errorf("device not found for volume %s", volumeID)
}

func (m *mounter) getDevicePathForVMware(ctx context.Context, volumeID string) (string, error) {
	logger := klog.FromContext(ctx)

	// Loop through /dev/sdb to /dev/sdz (/dev/sda -> the root disk)
	for i := 'b'; i <= 'z'; i++ {
		devicePath := fmt.Sprintf("/dev/sd%c", i)
		logger.V(5).Info("Checking VMware device path", "devicePath", devicePath, "volumeID", volumeID)

		if _, err := os.Stat(devicePath); err == nil {
			isBlock, err := m.IsBlockDevice(devicePath)
			if err == nil && isBlock {
				if m.verifyDevice(ctx, devicePath, volumeID) {
					logger.V(4).Info("Found and verified VMware device", "devicePath", devicePath, "volumeID", volumeID)
					return devicePath, nil
				}
			}
		}
	}
	return "", fmt.Errorf("device not found for volume %s", volumeID)
}

func (m *mounter) verifyDevice(ctx context.Context, devicePath string, volumeID string) bool {
	logger := klog.FromContext(ctx)

	size, err := m.GetBlockSizeBytes(devicePath)
	if err != nil {
		logger.V(4).Info("Failed to get device size", "devicePath", devicePath, "volumeID", volumeID, "error", err)
		return false
	}
	logger.V(5).Info("Device size retrieved", "devicePath", devicePath, "volumeID", volumeID, "sizeBytes", size)

	mounted, err := m.isDeviceMounted(devicePath)
	if err != nil {
		logger.V(4).Info("Failed to check if device is mounted", "devicePath", devicePath, "volumeID", volumeID, "error", err)
		return false
	}
	if mounted {
		logger.V(4).Info("Device is already mounted", "devicePath", devicePath, "volumeID", volumeID)
		return false
	}

	props, err := m.getDeviceProperties(devicePath)
	if err != nil {
		logger.V(4).Info("Failed to get device properties", "devicePath", devicePath, "volumeID", volumeID, "error", err)
		return false
	}
	logger.V(5).Info("Device properties retrieved", "devicePath", devicePath, "volumeID", volumeID, "properties", props)

	return true
}

func (m *mounter) isDeviceMounted(devicePath string) (bool, error) {
	output, err := m.Exec.Command("grep", devicePath, "/proc/mounts").Output()
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return len(output) > 0, nil
}

func (m *mounter) isDeviceInUse(devicePath string) (bool, error) {
	output, err := m.Exec.Command("lsof", devicePath).Output()
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return len(output) > 0, nil
}

func (m *mounter) getDeviceProperties(devicePath string) (map[string]string, error) {
	output, err := m.Exec.Command("udevadm", "info", "--query=property", devicePath).Output()
	if err != nil {
		return nil, err
	}

	props := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			props[parts[0]] = parts[1]
		}
	}

	return props, nil
}

func (m *mounter) probeVolume(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Scanning SCSI host")

	scsiPath := "/sys/class/scsi_host/"
	if dirs, err := os.ReadDir(scsiPath); err == nil {
		for _, f := range dirs {
			name := scsiPath + f.Name() + "/scan"
			data := []byte("- - -")
			logger.V(2).Info("Triggering SCSI host rescan")
			if err = os.WriteFile(name, data, 0o666); err != nil { //nolint:gosec
				logger.Error(err, "Failed to rescan scsi host ", "dirName", name)
			}
		}
	} else {
		logger.Error(err, "Failed to read dir ", "dirName", scsiPath)
	}

	args := []string{"trigger"}
	cmd := m.Exec.Command("udevadm", args...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(err, "Error running udevadm trigger")
	}
}

func (m *mounter) GetDeviceName(mountPath string) (string, int, error) {
	return mount.GetDeviceNameFromMount(m, mountPath)
}

// diskUUIDToSerial reproduces CloudStack function diskUuidToSerial
// from https://github.com/apache/cloudstack/blob/0f3f2a0937/plugins/hypervisors/kvm/src/main/java/com/cloud/hypervisor/kvm/resource/LibvirtComputingResource.java#L3000
//
// This is what CloudStack do *with KVM hypervisor* to translate
// a CloudStack volume UUID to libvirt disk serial.
func diskUUIDToSerial(uuid string) string {
	uuidWithoutHyphen := strings.ReplaceAll(uuid, "-", "")
	if len(uuidWithoutHyphen) < 20 {
		return uuidWithoutHyphen
	}

	return uuidWithoutHyphen[:20]
}

func (*mounter) PathExists(path string) (bool, error) {
	return mount.PathExists(path)
}

func (*mounter) MakeDir(pathname string) error {
	err := os.MkdirAll(pathname, os.FileMode(0o755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	return nil
}

func (*mounter) MakeFile(pathname string) error {
	f, err := os.OpenFile(pathname, os.O_CREATE, os.FileMode(0o644))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err = f.Close(); err != nil {
		return err
	}

	return nil
}

// Resize resizes the filesystem of the given devicePath.
func (m *mounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	return mount.NewResizeFs(m.Exec).Resize(devicePath, deviceMountPath)
}

// NeedResize checks if the filesystem of the given devicePath needs to be resized.
func (m *mounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	return mount.NewResizeFs(m.Exec).NeedResize(devicePath, deviceMountPath)
}

// GetStatistics gathers statistics on the volume.
func (m *mounter) GetStatistics(volumePath string) (volumeStatistics, error) {
	isBlock, err := m.IsBlockDevice(volumePath)
	if err != nil {
		return volumeStatistics{}, fmt.Errorf("failed to determine if volume %s is block device: %w", volumePath, err)
	}

	if isBlock {
		// See http://man7.org/linux/man-pages/man8/blockdev.8.html for details
		output, err := exec.Command("blockdev", "getsize64", volumePath).CombinedOutput()
		if err != nil {
			return volumeStatistics{}, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %w", volumePath, string(output), err)
		}
		strOut := strings.TrimSpace(string(output))
		gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
		if err != nil {
			return volumeStatistics{}, fmt.Errorf("failed to parse size %s into int", strOut)
		}

		return volumeStatistics{
			TotalBytes: gotSizeBytes,
		}, nil
	}

	var statfs unix.Statfs_t
	// See http://man7.org/linux/man-pages/man2/statfs.2.html for details.
	err = unix.Statfs(volumePath, &statfs)
	if err != nil {
		return volumeStatistics{}, err
	}

	volStats := volumeStatistics{
		AvailableBytes: int64(statfs.Bavail) * int64(statfs.Bsize),                         //nolint:unconvert
		TotalBytes:     int64(statfs.Blocks) * int64(statfs.Bsize),                         //nolint:unconvert
		UsedBytes:      (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize), //nolint:unconvert

		AvailableInodes: int64(statfs.Ffree),
		TotalInodes:     int64(statfs.Files),
		UsedInodes:      int64(statfs.Files) - int64(statfs.Ffree),
	}

	return volStats, nil
}

// IsBlockDevice checks if the given path is a block device.
func (m *mounter) IsBlockDevice(devicePath string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Stat(devicePath, &stat)
	if err != nil {
		return false, err
	}

	return (stat.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

// IsCorruptedMnt return true if err is about corrupted mount point.
func (m *mounter) IsCorruptedMnt(err error) bool {
	return mount.IsCorruptedMnt(err)
}

// Unpublish unmounts the given path.
func (m *mounter) Unpublish(path string) error {
	return m.Unstage(path)
}

// Unstage unmounts the given path.
func (m *mounter) Unstage(path string) error {
	return mount.CleanupMountPoint(path, m, true)
}
