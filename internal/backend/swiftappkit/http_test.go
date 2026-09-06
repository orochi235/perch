package swiftappkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// buildProbe compiles source against the runtime and returns the binary.
func buildProbe(t *testing.T, source string) string {
	t.Helper()
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Runtime.swift"), runtimeSwift, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.swift"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "probe")
	out, err := exec.Command(swiftc, "-o", bin,
		filepath.Join(dir, "Runtime.swift"), filepath.Join(dir, "main.swift")).CombinedOutput()
	if err != nil {
		t.Fatalf("compiling the probe: %v\n%s", err, out)
	}
	return bin
}

// httpProbe drives the http watch and the post action against a server the Go
// side controls. Both reach the network, so neither is exercised anywhere else:
// a request that never returns freezes the poll, and a post that sends the
// wrong body or content type fails silently at whatever it was aimed at.
const httpProbe = `
import Foundation

let base = ProcessInfo.processInfo.environment["PERCH_BASE"] ?? ""

let ok = Watcher.http(base + "/ok")
print("ok ok=\(ok.ok) status=\(ok.status) out=\(ok.out)")

let bad = Watcher.http(base + "/bad")
print("bad ok=\(bad.ok) status=\(bad.status) out=\(bad.out)")

print("empty=\(Watcher.http("").ok)|\(Watcher.http("").status)")

// A closed port answers immediately; the widget must not sit on it.
let refused = Watcher.http("http://127.0.0.1:1/nope")
print("refused ok=\(refused.ok) status=\(refused.status)")

// The request's own timeout has to fire well before the wait around it, or a
// slow host holds the poll open for the whole outer bound.
let started = Date()
let slow = Watcher.http(base + "/slow")
let elapsed = Date().timeIntervalSince(started)
print("slow ok=\(slow.ok) status=\(slow.status) under9=\(elapsed < 9)")

var posted = false
Act.post(base + "/post", body: "{\"source\":\"probe\"}") { posted = true }
let deadline = Date().addingTimeInterval(20)
while !posted && Date() < deadline {
    RunLoop.main.run(mode: .default, before: Date().addingTimeInterval(0.05))
}
print("post repolled=\(posted)")
`

func TestRuntimeHTTPAndPost(t *testing.T) {
	var mu sync.Mutex
	var postBody, postType, postMethod string

	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jobs":[]}`))
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(8 * time.Second):
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		mu.Lock()
		postBody, postType, postMethod = string(body), r.Header.Get("Content-Type"), r.Method
		mu.Unlock()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	bin := buildProbe(t, httpProbe)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, bin)
	c.Env = append(os.Environ(), "PERCH_BASE="+srv.URL)
	out, err := c.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("the runtime did not finish; a request that never returns freezes the poll\n%s", out)
	}
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}

	want := []string{
		`ok ok=true status=200 out={"jobs":[]}`,
		"bad ok=false status=500 out=boom",
		"empty=false|0",
		"refused ok=false status=0",
		"slow ok=false status=0 under9=true",
		"post repolled=true",
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

	mu.Lock()
	defer mu.Unlock()
	if postMethod != "POST" {
		t.Errorf("post used %s", postMethod)
	}
	if postType != "application/json" {
		t.Errorf("post Content-Type = %q, want application/json", postType)
	}
	if postBody != `{"source":"probe"}` {
		t.Errorf("post body = %q, want the body the item declared", postBody)
	}
}
