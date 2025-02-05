// Copyright 2019 Hewlett Packard Enterprise Development LP

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/hpe-storage/common-host-libs/dbservice/etcd"
	log "github.com/hpe-storage/common-host-libs/logger"
	"github.com/hpe-storage/common-host-libs/tunelinux"
	"github.com/hpe-storage/common-host-libs/util"

	"github.com/hpe-storage/csi-driver/pkg/driver"
	"github.com/hpe-storage/csi-driver/pkg/flavor"
)

const (
	csiVersion    = "0.1"
	csiDriverName = "csi.hpe.com"
	csiLogFile    = "/var/log/hpe-csi.log"
	csiEndpoint   = "unix:///var/lib/kubelet/csi.hpe.com/csi.sock"
)

var (
	// Flag variables for the command options
	name       string
	endpoint   string
	dbServer   string
	dbPort     string
	flavorName string

	// RootCmd is the main CSI command
	RootCmd = &cobra.Command{
		Use:              "csi",
		Short:            "HPE CSI command-line utility",
		Long:             `A command-line utility for managing the HPE Container Storage Interface (CSI) plugin`,
		TraverseChildren: true,
		Run: func(cmd *cobra.Command, args []string) {
			if err := csiCliHandler(cmd); err != nil {
				log.Errorf("Failed to execute CLI handler, Err: %v", err.Error())
				os.Exit(1)
			}
		},
	}
)

// Initialize cmd-line flags/commands
func init() {
	RootCmd.PersistentFlags().StringVarP(&name, "name", "n", csiDriverName, "CSI driver name")
	RootCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", csiEndpoint, "CSI endpoint")
	RootCmd.PersistentFlags().StringVarP(&dbServer, "dbserver", "s", "", "Database server for the CSI driver")
	RootCmd.PersistentFlags().StringVarP(&dbPort, "dbport", "p", etcd.DefaultPort, "Database server port for the CSI driver")
	RootCmd.PersistentFlags().BoolP("node-service", "", false, "CSI node-plugin")
	RootCmd.PersistentFlags().BoolP("help", "h", false, "Show help information")
	RootCmd.PersistentFlags().StringVarP(&flavorName, "flavor", "f", "", "CSI driver flavor")
}

func csiCliHandler(cmd *cobra.Command) error {
	log.Trace(">>>>> csiCliHandler")
	defer log.Trace("<<<<< csiCliHandler")

	// Process cmd-line arguments for the CSI driver
	driverName, _ := cmd.Flags().GetString("name")
	nodeService, _ := cmd.Flags().GetBool("node-service")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	dbServer, _ := cmd.Flags().GetString("dbserver")
	dbPort, _ := cmd.Flags().GetString("dbport")
	flavorName, _ := cmd.Flags().GetString("flavor")

	// Parse the endpoint
	_, addr, err := driver.ParseEndpoint(endpoint)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	// Check if the endpoint's dirpath exists
	dirPath := filepath.Dir(addr)
	exists, isDir, err := util.FileExists(dirPath)
	if err != nil {
		return fmt.Errorf("Error while processing the filepath %v, err: %v", dirPath, err.Error())
	}
	if !exists || !isDir {
		return fmt.Errorf("Directory path %v does not exist", dirPath)
	}

	// Set the flavor
	if flavorName == "" {
		flavorName = flavor.Vanilla
	}

	if nodeService {
		// perform conformance checks and service management
		// configure iscsi
		err = tunelinux.ConfigureIscsi()
		if err != nil {
			return fmt.Errorf("Unable to configure iscsid service, err %v", err.Error())
		}

		// configure multipath
		err = tunelinux.ConfigureMultipath()
		if err != nil {
			return fmt.Errorf("Unable to configure multipathd service, err %v", err.Error())
		}
	}

	log.Tracef("About to start the CSI driver '%v'", driverName)
	pid := os.Getpid()
	d, err := driver.NewDriver(
		driverName,
		csiVersion,
		endpoint,
		flavorName,
		nodeService,
		dbServer,
		dbPort)
	if err != nil {
		return fmt.Errorf("Error instantiating plugin %v, Err: %v", driverName, err.Error())
	}

	d.Start(nodeService)
	log.Infof("[%d] reply  : %v", pid, os.Args)

	// Handle signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt,
		syscall.SIGSEGV,
		syscall.SIGTERM)

	s := <-stop
	log.Fatalf("Exiting due to signal [%v] notification for pid [%d]", s.String(), pid)
	d.Stop(nodeService)
	log.Infof("Stopped [%d]", pid)
	return nil
}

// Main runs csi interpreting command-line flags and commands
func Main() {
	log.InitLogging(csiLogFile, nil, true)

	log.Trace(" **********************************************")
	log.Trace(" *************** HPE CSI DRIVER ***************")
	log.Trace(" **********************************************")

	log.Trace(">>>>> Main, os.Args:", os.Args)
	defer log.Trace("<<<<< Main")

	if err := RootCmd.Execute(); err != nil {
		log.Error("Failed to execute, err:", err.Error())
		os.Exit(1)
	}
}

func main() {
	/* Start CSI plugin service */
	Main()
}
