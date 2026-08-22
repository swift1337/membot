package system

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSystemdUnit(t *testing.T) {
	t.Parallel()

	rendered, err := renderSystemdUnit(InstallOptions{
		MembotPath: "/usr/local/bin/membot",
		DBPath:     "/home/test user/.membot/membot.db",
		Interval:   120 * time.Second,
	})
	if err != nil {
		t.Fatalf("renderSystemdUnit() error = %v", err)
	}

	checks := []string{
		`ExecStart="/usr/local/bin/membot" index all --watch`,
		`--interval "2m0s"`,
		`--db "/home/test user/.membot/membot.db"`,
		"Restart=on-failure",
		"WantedBy=default.target",
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("rendered unit missing %q:\n%s", check, rendered)
		}
	}
}

func TestSystemdQuoteEscapesSpecifiers(t *testing.T) {
	t.Parallel()

	if got, want := systemdQuote("/tmp/100% ready"), `"/tmp/100%% ready"`; got != want {
		t.Fatalf("systemdQuote() = %q, want %q", got, want)
	}
}

func TestIsDebianFamily(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		osRelease string
		want      bool
	}{
		{name: "debian", osRelease: "ID=debian\n", want: true},
		{name: "ubuntu", osRelease: "ID=ubuntu\nID_LIKE=debian\n", want: true},
		{name: "ubuntu derivative", osRelease: "ID=linuxmint\nID_LIKE=\"ubuntu debian\"\n", want: true},
		{name: "unrelated linux", osRelease: "ID=fedora\nID_LIKE=\"rhel centos\"\n", want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isDebianFamily(test.osRelease); got != test.want {
				t.Fatalf("isDebianFamily() = %v, want %v", got, test.want)
			}
		})
	}
}
