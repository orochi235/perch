package swiftappkit

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/orochi235/perch/internal/install"
)

// requireAqua skips outside a GUI login session. launchd has no gui/<uid>
// domain over SSH, and setActivationPolicy and WebKit both need a window server.
func requireAqua(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	out, err := exec.Command("launchctl", "managername").Output()
	if err != nil || strings.TrimSpace(string(out)) != "Aqua" {
		t.Skipf("not a GUI login session (%s)", out)
	}
}

// buildWindowProbe compiles a probe with Window.swift beside Runtime.swift, so
// it can drive WebWindow. buildProbe alone omits it: Window.swift is emitted
// only for a spec that declares a window.
func buildWindowProbe(t *testing.T, source string) string {
	t.Helper()
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	for name, body := range map[string][]byte{
		"Runtime.swift": runtimeSwift,
		"Window.swift":  windowSwift,
		"main.swift":    []byte(source),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "probe")
	out, err := exec.Command(swiftc, "-o", bin,
		filepath.Join(dir, "Runtime.swift"),
		filepath.Join(dir, "Window.swift"),
		filepath.Join(dir, "main.swift")).CombinedOutput()
	if err != nil {
		t.Fatalf("compiling the probe: %v\n%s", err, out)
	}
	return bin
}

// TestInstalledAgentStaysDownAfterACleanExit is the regression for KeepAlive.
// Every docs example ends with a Quit item; quitting exits 0, and an
// unconditional KeepAlive brought the widget straight back. This bootstraps the
// plist perch actually writes rather than one written for the test, so the
// assertion is about the shipped template.
func TestInstalledAgentStaysDownAfterACleanExit(t *testing.T) {
	requireAqua(t)

	dir := t.TempDir()
	label := "dev.perch.test.cleanexit." + strconv.Itoa(os.Getpid())
	domain := "gui/" + strconv.Itoa(os.Getuid())
	marker := filepath.Join(dir, "runs")

	// Appends a line and exits 0, the way a widget quit from its own menu does.
	program := filepath.Join(dir, "once.sh")
	script := fmt.Sprintf("#!/bin/sh\necho ran >> %q\nexit 0\n", marker)
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	plist := filepath.Join(dir, label+".plist")
	if err := os.WriteFile(plist, []byte(install.AgentPlist(label, program, install.InstallPATH())), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exec.Command("launchctl", "bootout", domain+"/"+label).Run() })
	if out, err := exec.Command("launchctl", "bootstrap", domain, plist).CombinedOutput(); err != nil {
		t.Fatalf("bootstrap: %v\n%s", err, out)
	}

	// RunAtLoad fires once; a restart loop would keep appending.
	time.Sleep(3 * time.Second)
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the agent never ran: %v", err)
	}
	if runs := strings.Count(string(body), "ran"); runs != 1 {
		t.Errorf("the agent ran %d times after exiting 0; launchd is restarting a clean exit", runs)
	}
}

// dockProbe mirrors what the generated Controller does with onVisibilityChange,
// and reports the activation policy at each step. Getting the order wrong does
// not error — it strands the menu bar or activates an app with no Dock tile.
const dockProbe = `
import AppKit

let app = NSApplication.shared
app.setActivationPolicy(.accessory)

func policy() -> String {
    switch NSApp.activationPolicy() {
    case .regular: return "regular"
    case .accessory: return "accessory"
    default: return "other"
    }
}

let window = WebWindow(
    url: "about:blank", title: "probe", width: 400, height: 300,
    zoom: nil, autosaveName: "ProbeWindow")
window.onVisibilityChange = { visible in
    if visible {
        NSApp.setActivationPolicy(.regular)
    } else {
        NSApp.setActivationPolicy(.accessory)
        NSApp.deactivate()
    }
}

print("launch=\(policy())")
print("presented=\(window.isPresented)")
window.show()
print("open=\(policy())")
print("openPresented=\(window.isPresented)")
window.hide()
print("closed=\(policy())")
print("closedPresented=\(window.isPresented)")
window.show()
print("reopen=\(policy())")
`

func TestDockTileFollowsTheWindow(t *testing.T) {
	requireAqua(t)
	bin := buildWindowProbe(t, dockProbe)
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	want := []string{
		"launch=accessory",
		"presented=false",
		"open=regular",
		"openPresented=true",
		"closed=accessory",
		"closedPresented=false",
		"reopen=regular",
	}
	got := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])
		}
	}
}

// reloadProbe opens, closes and reopens, then reports. What it costs the server
// is the assertion, and it is counted on the Go side.
const reloadProbe = `
import AppKit
import WebKit

let app = NSApplication.shared
app.setActivationPolicy(.accessory)

let url = CommandLine.arguments[1]
let window = WebWindow(
    url: url, title: "probe", width: 400, height: 300,
    zoom: nil, autosaveName: "ReloadProbeWindow")

func settle(_ seconds: TimeInterval) {
    let until = Date().addingTimeInterval(seconds)
    while Date() < until {
        RunLoop.main.run(mode: .default, before: Date().addingTimeInterval(0.05))
    }
}

window.show()
settle(3)
window.hide()
settle(0.5)
window.show()
settle(3)
print("done")
`

// TestReopenReloadsOnlyAfterAFailedLoad is the behavior the design doc called
// entangling — the reason embedded web views were out of scope. A page that
// loaded is reopened as it was, keeping scroll position; one that failed is
// fetched again, so a widget held open across a server restart does not show
// WebKit's error page forever.
func TestReopenReloadsOnlyAfterAFailedLoad(t *testing.T) {
	requireAqua(t)
	bin := buildWindowProbe(t, reloadProbe)

	t.Run("a page that loaded is not fetched again", func(t *testing.T) {
		var hits int64
		ln := serve(t, func(c net.Conn) {
			// Only the document: WebKit also asks for /favicon.ico, which is
			// not a reload of the page.
			if path(c) == "/" {
				atomic.AddInt64(&hits, 1)
			}
			body := "<html><title>ok</title><body>ok</body></html>"
			fmt.Fprintf(c, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
			c.Close()
		})
		run(t, bin, "http://"+ln.Addr().String()+"/")
		if got := atomic.LoadInt64(&hits); got != 1 {
			t.Errorf("the server was asked %d times; a successful page should survive a reopen untouched", got)
		}
	})

	t.Run("a page that failed is fetched again", func(t *testing.T) {
		var hits int64
		// Accept and hang up without answering: the navigation fails rather
		// than rendering an error status.
		ln := serve(t, func(c net.Conn) {
			if path(c) == "/" {
				atomic.AddInt64(&hits, 1)
			}
			c.Close()
		})
		run(t, bin, "http://"+ln.Addr().String()+"/")
		if got := atomic.LoadInt64(&hits); got < 2 {
			t.Errorf("the server was asked %d times; a failed page must be fetched again on reopen", got)
		}
	})
}

// path reads the request line and returns what was asked for, or "" if nothing
// arrived. A connection opened and dropped without a request is not a fetch.
func path(c net.Conn) string {
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return ""
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// serve accepts connections until the test ends, handing each to handle.
func serve(t *testing.T, handle func(net.Conn)) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handle(c)
		}
	}()
	return ln
}

func run(t *testing.T, bin string, args ...string) {
	t.Helper()
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "done") {
		t.Fatalf("the probe did not finish:\n%s", out)
	}
}
