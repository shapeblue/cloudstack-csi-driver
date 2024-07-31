// cloudstack-csi-driver binary.
//
// To get usage information:
//
//	cloudstack-csi-driver -h
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"

	"github.com/shapeblue/cloudstack-csi-driver/pkg/cloud"
	"github.com/shapeblue/cloudstack-csi-driver/pkg/driver"
)

func main() {
	fs := flag.NewFlagSet("cloudstack-csi-driver", flag.ExitOnError)
	if err := logsapi.RegisterLogFormat(logsapi.JSONLogFormat, json.Factory{}, logsapi.LoggingBetaOptions); err != nil {
		klog.ErrorS(err, "failed to register JSON log format")
	}

	var (
		showVersion = fs.Bool("version", false, "Show version")
		args        = os.Args[1:]
		cmd         = string(driver.AllMode)
		options     = driver.Options{}
	)
	fg := featuregate.NewFeatureGate()
	err := logsapi.AddFeatureGates(fg)
	if err != nil {
		klog.ErrorS(err, "failed to add feature gates")
	}

	c := logsapi.NewLoggingConfiguration()
	logsapi.AddFlags(c, fs)

	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		cmd = os.Args[1]
		args = os.Args[2:]
	}

	switch cmd {
	case string(driver.ControllerMode), string(driver.NodeMode), string(driver.AllMode):
		options.Mode = driver.Mode(cmd)
	default:
		klog.Errorf("Unknown driver mode %s: Expected %s, %s, or %s", cmd, driver.ControllerMode, driver.NodeMode, driver.AllMode)
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}

	options.AddFlags(fs)

	if err = fs.Parse(args); err != nil {
		klog.ErrorS(err, "Failed to parse options")
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}
	if err = options.Validate(); err != nil {
		klog.ErrorS(err, "Invalid options")
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}

	logs.InitLogs()
	logger := klog.Background()
	if err = logsapi.ValidateAndApply(c, fg); err != nil {
		logger.Error(err, "LoggingConfiguration is invalid")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if *showVersion {
		versionInfo, versionErr := driver.GetVersionJSON()
		if versionErr != nil {
			logger.Error(err, "failed to get version")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
		fmt.Println(versionInfo) //nolint:forbidigo
		os.Exit(0)
	}

	// Setup cloud connector.
	config, err := cloud.ReadConfig(options.CloudStackConfig)
	if err != nil {
		logger.Error(err, "Cannot read CloudStack configuration")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	logger.Info("Successfully read CloudStack configuration", "cloudstackconfig", options.CloudStackConfig)

	ctx := klog.NewContext(context.Background(), logger)
	csConnector := cloud.New(config)

	d, err := driver.New(ctx, csConnector, &options, nil)
	if err != nil {
		logger.Error(err, "Failed to initialize driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	if err = d.Run(ctx); err != nil {
		logger.Error(err, "Failed to run driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
