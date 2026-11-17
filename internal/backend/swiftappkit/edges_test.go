package swiftappkit

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/orochi235/perch/v2/internal/spec"
)

// edgesDoc holds expressions and shapes that once lowered cleanly and then
// failed in swiftc, or trapped at launch.
const edgesDoc = `
app: {name: edges, id: dev.edges.menubar, icon: circle, interval: 5s}
watch:
  w:
    run: [echo, '{"items":["a"]}']
    json: true
    shape:
      items: [string]
      jobs: [{name: string}]
      nothing: {}
      CodingKeys: string
  d:
    run: [echo, '{}']
    json: true
  e:
    run: [echo, '{}']
    json: true
    shape: {}
status:
  - badge: "w.data.items[0]"
menu:
  - {text: Ordered, when: "d.data.a > d.data.b"}
  - {text: "{{d.data.items[d.data.i]}}"}
  - {text: Found, when: "w.data.jobs[0] in w.data.jobs"}
  - {text: "{{(w.ok ? w.data.jobs[0] : w.data.jobs[1]).name}}"}
  - {text: "{{w.data.CodingKeys}}"}
  - text: Post
    post:
      url: http://localhost:3000
      body:
        message: '{{w.ok ? "up" : "down"}}'
        healthy: '{{w.code < 2 && w.ok}}'
  - {text: Quit, quit: true}
`

func TestEdgeCasesTypecheck(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	s, err := spec.Parse([]byte(edgesDoc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	files, err := New().Emit(s)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	typecheck(t, swiftc, files)
}

// The face is drawn from empty results before the first poll lands, so an
// index into a declared list reads an empty one.
func TestFaceDrawsBeforeTheFirstPoll(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	s, err := spec.Parse([]byte(edgesDoc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	probe := func(*spec.Spec, structNames) string {
		return "import Foundation\n" +
			"print(\"badge=\\(renderFace(Results()).badge)|\")\n" +
			"print(\"nodes=\\(renderMenu(Results()).count)\")\n"
	}
	files, err := emitFiles(s, runtimeFiles(), probe)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	bin := compileProbe(t, swiftc, files)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "badge=|") {
		t.Errorf("output = %q, want an empty badge", out)
	}
}
