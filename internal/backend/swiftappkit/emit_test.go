package swiftappkit

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/backend"
	"github.com/orochi235/perch/internal/spec"
)

func emit(t *testing.T, doc string) map[string]string {
	t.Helper()
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	files, err := New().Emit(s)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[f.Name] = string(f.Body)
	}
	return out
}

const minimal = `
app: {name: tiny, id: dev.tiny.menubar, icon: circle, interval: 3s}
menu: [{text: Quit, quit: true}]
`

func TestEmitProducesRuntimeAndApp(t *testing.T) {
	files := emit(t, minimal)
	for _, want := range []string{"Runtime.swift", "main.swift"} {
		if _, ok := files[want]; !ok {
			t.Errorf("no %s emitted; got %v", want, keys(files))
		}
	}
}

func TestEmitOmitsShapesWhenNoWatchDeclaresOne(t *testing.T) {
	if _, ok := emit(t, minimal)["Shapes.swift"]; ok {
		t.Error("Shapes.swift emitted for a spec with no shapes")
	}
}

func TestEmitEveryFileSaysRegeneratingOverwrites(t *testing.T) {
	for name, body := range emit(t, minimal) {
		if !strings.Contains(body, "Regenerating overwrites this file") {
			t.Errorf("%s has no generated-file header", name)
		}
	}
}

func TestEmitCarriesAppIdentityAndInterval(t *testing.T) {
	app := emit(t, minimal)["main.swift"]
	for _, want := range []string{`"circle"`, "3.0"} {
		if !strings.Contains(app, want) {
			t.Errorf("main.swift missing %s", want)
		}
	}
}

func TestBackendName(t *testing.T) {
	var b backend.Backend = New()
	if b.Name() != "swift-appkit" {
		t.Errorf("Name = %q", b.Name())
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// collidingNames is the spec that broke the old path-concatenated naming: field
// a_b and field b under a both flattened to FleetDataAB, and so did watches a_b
// and aB.
const collidingNames = `
app: {name: clash, id: dev.clash.menubar, icon: circle, interval: 3s}
watch:
  a_b: {run: [echo, x]}
  aB:
    run: [echo, y]
    json: true
    shape:
      a_b: {x: int}
      a: {b: {x: int}}
menu: [{text: "{{aB.data.a_b.x}}"}]
`

func TestEmitGivesEveryGeneratedStructItsOwnName(t *testing.T) {
	files := emit(t, collidingNames)
	seen := map[string]bool{}
	for _, body := range []string{files["Shapes.swift"], files["main.swift"]} {
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "struct ") {
				continue
			}
			name := strings.Fields(line)[1]
			name = strings.TrimSuffix(name, ":")
			if seen[name] {
				t.Errorf("struct %s declared twice", name)
			}
			seen[name] = true
		}
	}
}
