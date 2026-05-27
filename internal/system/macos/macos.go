package macos

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/swift1337/membot/internal/indexer"
	"github.com/swift1337/membot/internal/store"
)

const (
	Label           = "com.swift1337.membot.index"
	launchAgentName = Label + ".plist"
)

var ErrUnsupported = errors.New("Only MacOS service is supported") //nolint:staticcheck // stable CLI error text

//go:embed launchagent.plist.tmpl
var launchAgentTemplate string

type InstallOptions struct {
	MembotPath string
	DBPath     string
	Interval   time.Duration
}

type Status struct {
	Label            string `json:"label"`
	PlistPath        string `json:"plist_path"`
	Installed        bool   `json:"installed"`
	Loaded           bool   `json:"loaded"`
	StdoutPath       string `json:"stdout_path"`
	StderrPath       string `json:"stderr_path"`
	LaunchAgentPlist string `json:"launch_agent_plist,omitempty"`
}

func DefaultLaunchAgentPath() (string, error) {
	if err := requireDarwin(); err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentName), nil
}

func RenderLaunchAgent(opts InstallOptions) (string, error) {
	if err := requireDarwin(); err != nil {
		return "", err
	}
	if opts.MembotPath == "" {
		path, err := exec.LookPath("membot")
		if err != nil {
			return "", fmt.Errorf("resolve membot binary: %w", err)
		}
		opts.MembotPath = path
	}
	if opts.DBPath == "" {
		opts.DBPath = store.DefaultPath()
	}
	if opts.Interval <= 0 {
		opts.Interval = 120 * time.Second
	}

	logDir := indexer.DefaultLogDir()
	data := struct {
		Label      string
		MembotPath string
		Interval   string
		DBPath     string
		StdoutPath string
		StderrPath string
	}{
		Label:      Label,
		MembotPath: opts.MembotPath,
		Interval:   opts.Interval.String(),
		DBPath:     opts.DBPath,
		StdoutPath: filepath.Join(logDir, "index.stdout.log"),
		StderrPath: filepath.Join(logDir, "index.stderr.log"),
	}

	tmpl, err := template.New("launchagent").Parse(launchAgentTemplate)
	if err != nil {
		return "", fmt.Errorf("parse launch agent template: %w", err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render launch agent template: %w", err)
	}
	return rendered.String(), nil
}

func Install(opts InstallOptions) (Status, error) {
	if err := requireDarwin(); err != nil {
		return Status{}, err
	}

	plistPath, err := DefaultLaunchAgentPath()
	if err != nil {
		return Status{}, err
	}

	rendered, err := RenderLaunchAgent(opts)
	if err != nil {
		return Status{}, err
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return Status{}, fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	if err := os.MkdirAll(indexer.DefaultLogDir(), 0o700); err != nil {
		return Status{}, fmt.Errorf("create log directory: %w", err)
	}
	if err := os.WriteFile(plistPath, []byte(rendered), 0o644); err != nil {
		return Status{}, fmt.Errorf("write launch agent %s: %w", plistPath, err)
	}

	_ = runLaunchctl("unload", plistPath)
	if err := runLaunchctl("load", plistPath); err != nil {
		return Status{}, fmt.Errorf("load launch agent: %w", err)
	}

	return StatusForPlist(plistPath)
}

func Uninstall() (Status, error) {
	if err := requireDarwin(); err != nil {
		return Status{}, err
	}

	plistPath, err := DefaultLaunchAgentPath()
	if err != nil {
		return Status{}, err
	}

	status, err := StatusForPlist(plistPath)
	if err != nil {
		return Status{}, err
	}

	if status.Installed {
		if err := runLaunchctl("unload", plistPath); err != nil {
			return Status{}, fmt.Errorf("unload launch agent: %w", err)
		}
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return Status{}, fmt.Errorf("remove launch agent %s: %w", plistPath, err)
		}
	}

	return StatusForPlist(plistPath)
}

func StatusForPlist(plistPath string) (Status, error) {
	if err := requireDarwin(); err != nil {
		return Status{}, err
	}

	logDir := indexer.DefaultLogDir()
	status := Status{
		Label:      Label,
		PlistPath:  plistPath,
		StdoutPath: filepath.Join(logDir, "index.stdout.log"),
		StderrPath: filepath.Join(logDir, "index.stderr.log"),
	}

	info, err := os.Stat(plistPath)
	status.Installed = err == nil && !info.IsDir()
	if status.Installed {
		data, readErr := os.ReadFile(plistPath)
		if readErr == nil {
			status.LaunchAgentPlist = string(data)
		}
	}

	output, err := exec.Command("launchctl", "list").CombinedOutput()
	if err != nil {
		return status, nil
	}
	status.Loaded = strings.Contains(string(output), Label)
	return status, nil
}

func requireDarwin() error {
	if runtime.GOOS != "darwin" {
		return ErrUnsupported
	}
	return nil
}

func runLaunchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
