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

//go:build sanity

package sanity

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/klog/v2"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	"github.com/shapeblue/cloudstack-csi-driver/pkg/cloud/fake"
	"github.com/shapeblue/cloudstack-csi-driver/pkg/driver"
	"github.com/shapeblue/cloudstack-csi-driver/pkg/mount"
)

func TestSanity(t *testing.T) {
	// Setup driver
	dir, err := ioutil.TempDir("", "sanity-cloudstack-csi")
	if err != nil {
		t.Fatalf("error creating directory: %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "target")
	stagingPath := filepath.Join(dir, "staging")
	endpoint := "unix://" + filepath.Join(dir, "csi.sock")

	config := sanity.NewTestConfig()
	config.TargetPath = targetPath
	config.StagingPath = stagingPath
	config.Address = endpoint
	config.TestVolumeParameters = map[string]string{
		driver.DiskOfferingKey: "9743fd77-0f5d-4ef9-b2f8-f194235c769c",
	}
	config.IdempotentCount = 5
	config.TestNodeVolumeAttachLimit = true

	logger := klog.Background()
	ctx := klog.NewContext(context.Background(), logger)

	options := driver.Options{
		Mode:     driver.AllMode,
		Endpoint: endpoint,
		NodeName: "node",
	}
	csiDriver, err := driver.New(ctx, fake.New(), &options, mount.NewFake())
	if err != nil {
		t.Fatalf("error creating driver: %v", err)
	}
	go func() {
		csiDriver.Run(ctx)
	}()

	sanity.Test(t, config)
}
