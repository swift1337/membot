package macos

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/swift1337/membot/internal/store"
)

func TestRenderLaunchAgent(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	rendered, err := RenderLaunchAgent(InstallOptions{
		MembotPath: "/usr/local/bin/membot",
		DBPath:     store.DefaultPath(),
		Interval:   120 * time.Second,
	})
	if err != nil {
		t.Fatalf("RenderLaunchAgent() error = %v", err)
	}

	checks := []string{
		"<string>com.swift1337.membot.index</string>",
		"<string>/usr/local/bin/membot</string>",
		"<string>index</string>",
		"<string>all</string>",
		"<string>--watch</string>",
		"<string>--interval</string>",
		"<string>2m0s</string>",
		"<string>--db</string>",
		"<string>" + store.DefaultPath() + "</string>",
		"<key>KeepAlive</key>",
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("rendered plist missing %q:\n%s", check, rendered)
		}
	}
}

func TestRequireDarwin(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "darwin" {
		if err := requireDarwin(); err != nil {
			t.Fatalf("requireDarwin() on darwin = %v, want nil", err)
		}
		return
	}

	if err := requireDarwin(); err != ErrUnsupported {
		t.Fatalf("requireDarwin() = %v, want ErrUnsupported", err)
	}
}
