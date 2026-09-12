// Package install owns the wrapper around the generated Swift: the app bundle,
// LSUIElement, the LaunchAgent plist, and the bootout-wait-bootstrap loop.
// This is the part each consuming repo used to reimplement in bash.
package install

import (
	"os"
	"strings"
	"text/template"
)

// App is the bundle identity install writes into Info.plist.
type App struct {
	Name       string
	ID         string
	Executable string
	// Identity is the keychain code signing identity, or empty for ad-hoc.
	Identity string
}

const plistHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
`

var infoTmpl = template.Must(template.New("info").Parse(plistHeader + `<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>{{.Name}}</string>
	<key>CFBundleDisplayName</key>
	<string>{{.Name}}</string>
	<key>CFBundleIdentifier</key>
	<string>{{.ID}}</string>
	<key>CFBundleExecutable</key>
	<string>{{.Executable}}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSUIElement</key>
	<true/>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
</dict>
</plist>
`))

// InfoPlist renders the bundle's Info.plist. The bundle exists at all because
// UNUserNotificationCenter refuses to run outside one; LSUIElement is what
// keeps a status-bar app out of the Dock.
func InfoPlist(a App) string {
	var sb strings.Builder
	_ = infoTmpl.Execute(&sb, App{
		Name:       escape(a.Name),
		ID:         escape(a.ID),
		Executable: escape(a.Executable),
	})
	return sb.String()
}

var agentTmpl = template.Must(template.New("agent").Parse(plistHeader + `<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Program}}</string>
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>{{.Path}}</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`))

// AgentPlist renders the LaunchAgent that keeps the app running. It carries a
// PATH because launchd's default cannot resolve a bare command in run:, and a
// watch that cannot resolve its command just leaves the widget looking broken.
func AgentPlist(label, program, path string) string {
	var sb strings.Builder
	_ = agentTmpl.Execute(&sb, struct{ Label, Program, Path string }{
		escape(label), escape(program), escape(path),
	})
	return sb.String()
}

// fallbackPATH is only reached when the installing process has none itself.
const fallbackPATH = "/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"

// InstallPATH is the PATH the agent runs with: the installing process's own, so
// that perch run and the installed agent resolve commands identically.
func InstallPATH() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return fallbackPATH
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
