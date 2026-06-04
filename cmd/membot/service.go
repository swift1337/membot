package main

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/system/macos"
)

const defaultServiceInterval = 120 * time.Second

var cmdService = &cobra.Command{
	Use:   "service",
	Short: "Manage the macOS background indexing service",
}

var cmdServiceInstall = &cobra.Command{
	Use:   "install",
	Short: "Install and load the macOS LaunchAgent",
	RunE:  runServiceInstall,
}

var cmdServiceUninstall = &cobra.Command{
	Use:   "uninstall",
	Short: "Unload and remove the macOS LaunchAgent",
	RunE:  runServiceUninstall,
}

var cmdServiceStatus = &cobra.Command{
	Use:   "status",
	Short: "Show macOS LaunchAgent status",
	RunE:  runServiceStatus,
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	status, err := macos.Install(macos.InstallOptions{
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
	status, err := macos.Uninstall()
	if err != nil {
		return err
	}
	return writeJSON(cmd, status)
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	plistPath, err := macos.DefaultLaunchAgentPath()
	if err != nil {
		return err
	}
	status, err := macos.StatusForPlist(plistPath)
	if err != nil {
		return err
	}
	return writeJSON(cmd, status)
}
