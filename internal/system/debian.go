package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/swift1337/membot/internal/store"
)

const (
	systemdUnitName = "membot.service"
	defaultInterval = 120 * time.Second
)

const systemdUnitTemplate = `[Unit]
Description=membot background indexer

[Service]
Type=simple
ExecStart={{quote .MembotPath}} index all --watch --interval {{quote .Interval}} --db {{quote .DBPath}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

func installDebian(opts InstallOptions) (Status, error) {
	if err := requireDebian(); err != nil {
		return Status{}, err
	}

	unitPath, err := defaultSystemdUnitPath()
	if err != nil {
		return Status{}, err
	}
	rendered, err := renderSystemdUnit(opts)
	if err != nil {
		return Status{}, err
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return Status{}, fmt.Errorf("create systemd user directory: %w", err)
	}
	if err := os.WriteFile(unitPath, []byte(rendered), 0o644); err != nil {
		return Status{}, fmt.Errorf("write systemd unit %s: %w", unitPath, err)
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return Status{}, err
	}
	if err := runSystemctl("enable", systemdUnitName); err != nil {
		return Status{}, err
	}
	if err := runSystemctl("restart", systemdUnitName); err != nil {
		return Status{}, err
	}

	return statusDebian()
}

func uninstallDebian() (Status, error) {
	if err := requireDebian(); err != nil {
		return Status{}, err
	}

	unitPath, err := defaultSystemdUnitPath()
	if err != nil {
		return Status{}, err
	}
	status, err := statusDebian()
	if err != nil {
		return Status{}, err
	}
	if status.Installed {
		if err := runSystemctl("disable", "--now", systemdUnitName); err != nil {
			return Status{}, err
		}
		if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
			return Status{}, fmt.Errorf("remove systemd unit %s: %w", unitPath, err)
		}
		if err := runSystemctl("daemon-reload"); err != nil {
			return Status{}, err
		}
	}

	return statusDebian()
}

func statusDebian() (Status, error) {
	if err := requireDebian(); err != nil {
		return Status{}, err
	}

	unitPath, err := defaultSystemdUnitPath()
	if err != nil {
		return Status{}, err
	}
	status := Status{
		Unit:     systemdUnitName,
		UnitPath: unitPath,
	}

	info, statErr := os.Stat(unitPath)
	status.Installed = statErr == nil && !info.IsDir()
	if status.Installed {
		data, readErr := os.ReadFile(unitPath)
		if readErr == nil {
			status.SystemdUnit = string(data)
		}
	}

	status.Loaded = exec.Command("systemctl", "--user", "is-active", "--quiet", systemdUnitName).Run() == nil
	return status, nil
}

func defaultSystemdUnitPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "systemd", "user", systemdUnitName), nil
}

func renderSystemdUnit(opts InstallOptions) (string, error) {
	if opts.MembotPath == "" {
		path, err := exec.LookPath("membot")
		if err != nil {
			return "", fmt.Errorf("resolve membot binary: %w", err)
		}
		opts.MembotPath = path
	}
	absolutePath, err := filepath.Abs(opts.MembotPath)
	if err != nil {
		return "", fmt.Errorf("resolve absolute membot path: %w", err)
	}
	opts.MembotPath = absolutePath
	if opts.DBPath == "" {
		opts.DBPath = store.DefaultPath()
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultInterval
	}

	data := struct {
		MembotPath string
		Interval   string
		DBPath     string
	}{
		MembotPath: opts.MembotPath,
		Interval:   opts.Interval.String(),
		DBPath:     opts.DBPath,
	}
	tmpl, err := template.New("systemd-unit").Funcs(template.FuncMap{
		"quote": systemdQuote,
	}).Parse(systemdUnitTemplate)
	if err != nil {
		return "", fmt.Errorf("parse systemd unit template: %w", err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render systemd unit template: %w", err)
	}
	return rendered.String(), nil
}

func systemdQuote(value string) string {
	return strings.ReplaceAll(strconv.Quote(value), "%", "%%")
}

func requireDebian() error {
	if runtime.GOOS != "linux" {
		return ErrUnsupported
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("detect Linux distribution: %w", err)
	}
	if !isDebianFamily(string(data)) {
		return ErrUnsupported
	}
	return nil
}

func isDebianFamily(osRelease string) bool {
	for line := range strings.Lines(osRelease) {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || (key != "ID" && key != "ID_LIKE") {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		for distro := range strings.FieldsSeq(value) {
			if distro == "debian" || distro == "ubuntu" {
				return true
			}
		}
	}
	return false
}

func runSystemctl(args ...string) error {
	commandArgs := append([]string{"--user"}, args...)
	output, err := exec.Command("systemctl", commandArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"systemctl %s: %w: %s",
			strings.Join(commandArgs, " "),
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return nil
}
