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

package mount

import (
	"context"
	"os"

	"k8s.io/mount-utils"
	exec "k8s.io/utils/exec/testing"
)

const (
	giB = 1 << 30
)

type fakeMounter struct {
	mount.SafeFormatAndMount
}

// NewFake creates a fake implementation of the
// mount.Interface, to be used in tests.
func NewFake() Interface {
	return &fakeMounter{
		mount.SafeFormatAndMount{
			Interface: mount.NewFakeMounter([]mount.MountPoint{}),
			Exec:      &exec.FakeExec{DisableScripts: true},
		},
	}
}

func (m *fakeMounter) GetBlockSizeBytes(_ string) (int64, error) {
	return 1073741824, nil
}

func (m *fakeMounter) GetDevicePath(_ context.Context, _ string) (string, error) {
	return "/dev/sdb", nil
}

func (m *fakeMounter) GetDeviceName(mountPath string) (string, int, error) {
	return mount.GetDeviceNameFromMount(m, mountPath)
}

func (*fakeMounter) PathExists(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (*fakeMounter) MakeDir(pathname string) error {
	err := os.MkdirAll(pathname, os.FileMode(0o755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	return nil
}

func (*fakeMounter) MakeFile(pathname string) error {
	file, err := os.OpenFile(pathname, os.O_CREATE, os.FileMode(0o644))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err = file.Close(); err != nil {
		return err
	}

	return nil
}

func (m *fakeMounter) GetStatistics(_ string) (volumeStatistics, error) {
	return volumeStatistics{
		AvailableBytes: 3 * giB,
		TotalBytes:     10 * giB,
		UsedBytes:      7 * giB,

		AvailableInodes: 3000,
		TotalInodes:     10000,
		UsedInodes:      7000,
	}, nil
}

func (m *fakeMounter) IsBlockDevice(_ string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) IsCorruptedMnt(_ error) bool {
	return false
}

func (m *fakeMounter) NeedResize(_ string, _ string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) Resize(_ string, _ string) (bool, error) {
	return true, nil
}

func (m *fakeMounter) Unpublish(path string) error {
	return m.Unstage(path)
}

func (m *fakeMounter) Unstage(path string) error {
	return mount.CleanupMountPoint(path, m, true)
}
