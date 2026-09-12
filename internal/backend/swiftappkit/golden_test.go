package swiftappkit

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestGolden pins YAML in against Swift out. TestEmittedSwiftTypechecks is its
// necessary companion: a golden alone stays green while emitting Swift that
// does not compile.
func TestGolden(t *testing.T) {
	for name, doc := range map[string]string{
		"minimal":       minimal,
		"design-doc":    designDocExample,
		"every-feature": everyFeature,
		"states":        states,
	} {
		t.Run(name, func(t *testing.T) {
			s, err := spec.Parse([]byte(doc))
			if err != nil {
				t.Fatalf("spec.Parse: %v", err)
			}
			files, err := New().Emit(s)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			for _, f := range files {
				if f.Name == "Runtime.swift" {
					continue // fixed, not generated from the spec
				}
				path := filepath.Join("testdata", name, f.Name)
				if *update {
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, f.Body, 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("%v (run go test ./... -update to create it)", err)
				}
				if string(f.Body) != string(want) {
					t.Errorf("%s differs from its golden; run go test ./... -update to see the change", path)
				}
			}
		})
	}
}
