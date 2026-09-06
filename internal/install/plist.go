// Package install owns the wrapper around the generated Swift: the app bundle,
// LSUIElement, the LaunchAgent plist, and the bootout-wait-bootstrap loop.
// This is the part each consuming repo used to reimplement in bash.
package install

import (
	"strings"
	"text/template"
)

// App is the bundle identity install writes into Info.plist.
type App struct {
	Name       string
	ID         string
	Executable string
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
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`))

// AgentPlist renders the LaunchAgent that keeps the app running.
func AgentPlist(label, program string) string {
	var sb strings.Builder
	_ = agentTmpl.Execute(&sb, struct{ Label, Program string }{escape(label), escape(program)})
	return sb.String()
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
