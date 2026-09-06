package install

import (
	"strings"
	"testing"
)

func TestInfoPlistMarksTheAppAsAgent(t *testing.T) {
	got := InfoPlist(App{Name: "onto", ID: "dev.onto.menubar", Executable: "onto"})
	for _, want := range []string{
		"<key>LSUIElement</key>",
		"<key>CFBundleIdentifier</key>",
		"<string>dev.onto.menubar</string>",
		"<string>onto</string>",
		"<key>CFBundlePackageType</key>",
		"<string>APPL</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Info.plist missing %s\n%s", want, got)
		}
	}
	// LSUIElement is what keeps the app out of the Dock and the app switcher.
	if !strings.Contains(got, "<key>LSUIElement</key>\n\t<true/>") {
		t.Errorf("LSUIElement is not set true:\n%s", got)
	}
}

func TestAgentPlistRunsTheBundledBinary(t *testing.T) {
	got := AgentPlist("dev.onto.menubar", "/Users/x/Applications/onto.app/Contents/MacOS/onto", "/usr/bin")
	for _, want := range []string{
		"<string>dev.onto.menubar</string>",
		"<string>/Users/x/Applications/onto.app/Contents/MacOS/onto</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("agent plist missing %s\n%s", want, got)
		}
	}
}

func TestPlistEscapesText(t *testing.T) {
	got := InfoPlist(App{Name: "a&b", ID: "x<y", Executable: "e"})
	if strings.Contains(got, "a&b") || strings.Contains(got, "x<y") {
		t.Errorf("plist did not escape XML text:\n%s", got)
	}
	if !strings.Contains(got, "a&amp;b") || !strings.Contains(got, "x&lt;y") {
		t.Errorf("plist escaping is wrong:\n%s", got)
	}
}

// A LaunchAgent inherits launchd's minimal PATH, so a bare command in run:
// cannot resolve and the widget sits there looking broken. The plist is the
// only artifact written per machine and never committed, so PATH belongs here.
func TestAgentPlistCarriesAPath(t *testing.T) {
	got := AgentPlist("dev.onto.menubar", "/x/onto", "/Users/x/.local/bin:/usr/bin")
	for _, want := range []string{
		"<key>EnvironmentVariables</key>",
		"<key>PATH</key>",
		"<string>/Users/x/.local/bin:/usr/bin</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("agent plist missing %s\n%s", want, got)
		}
	}
}

func TestAgentPlistEscapesThePath(t *testing.T) {
	got := AgentPlist("dev.x", "/x", "/a&b:/c")
	if strings.Contains(got, "/a&b") {
		t.Errorf("PATH was not escaped:\n%s", got)
	}
}

// perch run execs the binary with perch's own environment, so the installed
// agent has to resolve commands the same way or the two diverge.
func TestInstallPATHMatchesTheInstallingProcess(t *testing.T) {
	t.Setenv("PATH", "/custom/bin:/usr/bin")
	if got := InstallPATH(); got != "/custom/bin:/usr/bin" {
		t.Errorf("InstallPATH = %q, want the installing process's PATH", got)
	}
}

func TestInstallPATHFallsBackWhenUnset(t *testing.T) {
	t.Setenv("PATH", "")
	got := InstallPATH()
	if got == "" {
		t.Fatal("InstallPATH returned empty; a plist with no PATH is the bug this exists to prevent")
	}
	if !strings.Contains(got, "/usr/bin") {
		t.Errorf("fallback PATH = %q, want the system directories", got)
	}
}
