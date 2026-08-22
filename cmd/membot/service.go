package main

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/system"
)

const defaultServiceInterval = 120 * time.Second

var cmdService = &cobra.Command{
	Use:   "service",
	Short: "Manage the background indexing service",
}

var cmdServiceInstall = &cobra.Command{
	Use:   "install",
	Short: "Install and start the background indexing service",
	RunE:  runServiceInstall,
}

var cmdServiceUninstall = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and remove the background indexing service",
	RunE:  runServiceUninstall,
}

var cmdServiceStatus = &cobra.Command{
	Use:   "status",
	Short: "Show background indexing service status",
	RunE:  runServiceStatus,
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	status, err := system.Install(system.InstallOptions{
		MembotPath: runtimeConfig.Service.MembotPath,
		DBPath:     runtimeConfig.DBPath,
		Interval:   runtimeConfig.Service.Interval,
	})
	if err != nil {
		return err
	}
	return writeJSON(cmd, status)
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	status, err := system.Uninstall()
	if err != nil {
		return err
	}
	return writeJSON(cmd, status)
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	status, err := system.ServiceStatus()
	if err != nil {
		return err
	}
	return writeJSON(cmd, status)
}
