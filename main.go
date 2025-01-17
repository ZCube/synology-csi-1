/*
 * Copyright 2021 Synology Inc.
 */

package main

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/SynologyOpenSource/synology-csi/pkg/driver"
	"github.com/SynologyOpenSource/synology-csi/pkg/dsm/common"
	"github.com/SynologyOpenSource/synology-csi/pkg/dsm/service"
	"github.com/SynologyOpenSource/synology-csi/pkg/logger"
	"github.com/SynologyOpenSource/synology-csi/pkg/models"
)

var (
	// CSI options
	csiNodeID           = "CSINode"
	csiEndpoint         = "unix:///var/lib/kubelet/plugins/" + driver.DriverName + "/csi.sock"
	csiClientInfoPath   = "/etc/synology/client-info.yml"
	fsGroupChangePolicy = "OnRootMismatch"

	// Logging
	logLevel       = "info"
	webapiDebug    = false
	multipathForUC = true
)

var rootCmd = &cobra.Command{
	Use:          "synology-csi-driver",
	Short:        "Synology CSI Driver",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if webapiDebug {
			logger.WebapiDebug = true
			logLevel = "debug"
		}
		logger.Init(logLevel)

		if !multipathForUC {
			driver.MultipathEnabled = false
		}

		err := driverStart()
		if err != nil {
			log.Errorf("Failed to driverStart(): %v", err)
			return err
		}
		return nil
	},
}

func driverStart() error {
	log.Infof("CSI Options = {%s, %s, %s}", csiNodeID, csiEndpoint, csiClientInfoPath)

	// 1. Compile templates
	err := models.CompileTemplates()
	if err != nil {
		log.Errorf("Failed to compile templates: %v", err)
		return err
	}

	dsmService := service.NewDsmService()

	// 2. Login DSMs by given ClientInfo
	info, err := common.LoadConfig(csiClientInfoPath)
	if err != nil {
		log.Errorf("Failed to read config: %v", err)
		return err
	}

	for _, client := range info.Clients {
		err := dsmService.AddDsm(client)
		if err != nil {
			log.Errorf("Failed to add DSM: %s, error: %v", client.Host, err)
		}
	}
	defer dsmService.RemoveAllDsms()

	// 3. Create and Run the Driver
	drv, err := driver.NewControllerAndNodeDriver(csiNodeID, csiEndpoint, fsGroupChangePolicy, dsmService)
	if err != nil {
		log.Errorf("Failed to create driver: %v", err)
		return err
	}
	drv.Activate()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	// Block until a signal is received.
	<-c
	log.Infof("Shutting down.")
	return nil
}

func main() {
	addFlags(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

	os.Exit(0)
}

func addFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&csiNodeID, "nodeid", csiNodeID, "Node ID")
	cmd.PersistentFlags().StringVarP(&csiEndpoint, "endpoint", "e", csiEndpoint, "CSI endpoint")
	cmd.PersistentFlags().StringVarP(&csiClientInfoPath, "client-info", "f", csiClientInfoPath, "Path of Synology config yaml file")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", logLevel, "Log level (debug, info, warn, error, fatal)")
	cmd.PersistentFlags().BoolVarP(&webapiDebug, "debug", "d", webapiDebug, "Enable webapi debugging logs")
	cmd.PersistentFlags().BoolVar(&multipathForUC, "multipath", multipathForUC, "Set to 'false' to disable multipath for UC")
	cmd.PersistentFlags().StringVar(&fsGroupChangePolicy, "fsgroup-change-policy", fsGroupChangePolicy, "Set FSGroupChangePolicy for PVCs (Valid values: OnRootMismatch, Always, None)")
	cmd.PersistentFlags().StringVar(&models.TargetPrefix, "iscsi-target-prefix", models.TargetPrefix, "Set iscsi target prefix")
	cmd.PersistentFlags().StringVar(&models.IqnPrefix, "iscsi-iqn-prefix", models.IqnPrefix, "Set iscsi iqn prefix")
	cmd.PersistentFlags().StringVar(&models.LunPrefix, "iscsi-lun-prefix", models.LunPrefix, "Set iscsi lun prefix")
	cmd.PersistentFlags().StringVar(&models.SharePrefix, "share-prefix", models.SharePrefix, "Set share folder prefix")
	cmd.PersistentFlags().StringVar(&models.LunNameTemplate, "lun-name-template", models.LunNameTemplate, "Set lun name template")
	cmd.PersistentFlags().StringVar(&models.ShareNameTemplate, "share-name-template", models.ShareNameTemplate, "Set share folder name template")
	cmd.PersistentFlags().StringVar(&models.LunDescriptionTemplate, "lun-description-template", models.LunDescriptionTemplate, "Set lun description template")
	cmd.PersistentFlags().StringVar(&models.ShareDescriptionTemplate, "share-description-template", models.ShareDescriptionTemplate, "Set share folder description template")
	cmd.PersistentFlags().StringVar(&models.LunSnapshotNameTemplate, "lun-snapshot-name-template", models.LunSnapshotNameTemplate, "Set lun snapshot name template")
	cmd.PersistentFlags().StringVar(&models.ShareSnapshotNameTemplate, "share-snapshot-name-template", models.ShareSnapshotNameTemplate, "Set share folder snapshot name template")
	cmd.PersistentFlags().StringVar(&models.SnapshotDescriptionTemplate, "snapshot-description-template", models.SnapshotDescriptionTemplate, "Set snapshot description template")

	cmd.MarkFlagRequired("endpoint")
	cmd.MarkFlagRequired("client-info")
	cmd.Flags().SortFlags = false
	cmd.PersistentFlags().SortFlags = false
}
