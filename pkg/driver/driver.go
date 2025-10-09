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

// Package driver provides the implementation of the CSI plugin.
//
// It contains the gRPC server implementation of CSI specification.
package driver

import (
	"context"
	"fmt"
	"net"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	"github.com/cloudstack/cloudstack-csi-driver/pkg/cloud"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/mount"
	"github.com/cloudstack/cloudstack-csi-driver/pkg/util"
)

// Interface is the CloudStack CSI driver interface.
type Interface interface {
	// Run the CSI driver gRPC server
	Run(ctx context.Context) error
}

type cloudstackDriver struct {
	controller csi.ControllerServer
	node       csi.NodeServer
	options    *Options
}

// New instantiates a new CloudStack CSI driver.
func New(ctx context.Context, csConnector cloud.Interface, options *Options, mounter mount.Interface) (Interface, error) {
	logger := klog.FromContext(ctx)
	logger.Info("Driver starting", "Driver", DriverName, "Version", driverVersion)

	if err := validateMode(options.Mode); err != nil {
		return nil, fmt.Errorf("invalid driver options: %w", err)
	}

	driver := &cloudstackDriver{
		options: options,
	}

	switch options.Mode {
	case ControllerMode:
		driver.controller = NewControllerServer(csConnector)
	case NodeMode:
		driver.node = NewNodeServer(csConnector, mounter, options)
	case AllMode:
		driver.controller = NewControllerServer(csConnector)
		driver.node = NewNodeServer(csConnector, mounter, options)
	default:
		return nil, fmt.Errorf("unknown mode: %s", options.Mode)
	}

	return driver, nil
}

func (cs *cloudstackDriver) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	scheme, addr, err := util.ParseEndpoint(cs.options.Endpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Log every request and payloads (request + response)
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			resp, err := handler(klog.NewContext(ctx, logger), req)
			if err != nil {
				logger.Error(err, "GRPC method failed", "method", info.FullMethod)
			}

			return resp, err
		}),
	}
	grpcServer := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(grpcServer, cs)
	switch cs.options.Mode {
	case ControllerMode:
		csi.RegisterControllerServer(grpcServer, cs.controller)
	case NodeMode:
		csi.RegisterNodeServer(grpcServer, cs.node)
	case AllMode:
		csi.RegisterControllerServer(grpcServer, cs.controller)
		csi.RegisterNodeServer(grpcServer, cs.node)
	default:
		return fmt.Errorf("unknown mode: %s", cs.options.Mode)
	}

	logger.Info("Listening for connections", "address", listener.Addr())

	return grpcServer.Serve(listener)
}

func validateMode(mode Mode) error {
	if mode != AllMode && mode != ControllerMode && mode != NodeMode {
		return fmt.Errorf("mode is not supported (actual: %s, supported: %v)", mode, []Mode{AllMode, ControllerMode, NodeMode})
	}

	return nil
}
