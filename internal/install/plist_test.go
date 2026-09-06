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
	got := AgentPlist("dev.onto.menubar", "/Users/x/Applications/onto.app/Contents/MacOS/onto")
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
