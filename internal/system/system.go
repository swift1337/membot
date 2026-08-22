package system

import (
	"errors"
	"runtime"
	"time"

	"github.com/swift1337/membot/internal/system/macos"
)

var ErrUnsupported = errors.New("background service is supported only on macOS and Debian/Ubuntu Linux")

type InstallOptions struct {
	MembotPath string
	DBPath     string
	Interval   time.Duration
}

type Status struct {
	Label            string `json:"label,omitempty"`
	PlistPath        string `json:"plist_path,omitempty"`
	Unit             string `json:"unit,omitempty"`
	UnitPath         string `json:"unit_path,omitempty"`
	Installed        bool   `json:"installed"`
	Loaded           bool   `json:"loaded"`
	StdoutPath       string `json:"stdout_path,omitempty"`
	StderrPath       string `json:"stderr_path,omitempty"`
	LaunchAgentPlist string `json:"launch_agent_plist,omitempty"`
	SystemdUnit      string `json:"systemd_unit,omitempty"`
}

func Install(opts InstallOptions) (Status, error) {
	switch runtime.GOOS {
	case "darwin":
		status, err := macos.Install(macos.InstallOptions(opts))
		return fromMacOS(status), err
	case "linux":
		return installDebian(opts)
	default:
		return Status{}, ErrUnsupported
	}
}

func Uninstall() (Status, error) {
	switch runtime.GOOS {
	case "darwin":
		status, err := macos.Uninstall()
		return fromMacOS(status), err
	case "linux":
		return uninstallDebian()
	default:
		return Status{}, ErrUnsupported
	}
}

func ServiceStatus() (Status, error) {
	switch runtime.GOOS {
	case "darwin":
		plistPath, err := macos.DefaultLaunchAgentPath()
		if err != nil {
			return Status{}, err
		}
		status, err := macos.StatusForPlist(plistPath)
		return fromMacOS(status), err
	case "linux":
		return statusDebian()
	default:
		return Status{}, ErrUnsupported
	}
}

func fromMacOS(status macos.Status) Status {
	return Status{
		Label:            status.Label,
		PlistPath:        status.PlistPath,
		Installed:        status.Installed,
		Loaded:           status.Loaded,
		StdoutPath:       status.StdoutPath,
		StderrPath:       status.StderrPath,
		LaunchAgentPlist: status.LaunchAgentPlist,
	}
}
