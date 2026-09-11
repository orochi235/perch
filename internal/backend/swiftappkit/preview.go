package swiftappkit

import (
	_ "embed"
	"fmt"

	"github.com/orochi235/perch/internal/backend"
	"github.com/orochi235/perch/internal/celswift"
	"github.com/orochi235/perch/internal/spec"
)

//go:embed runtime/Preview.swift
var previewSwift []byte

// EmitPreview renders the same app with a driver where its main would be: it
// reads sample outcomes on stdin and prints what the status item and menu would
// show. The docs site draws its previews by running this, so a page cannot
// disagree with the app about what a menubar.yaml builds.
func (*Backend) EmitPreview(s *spec.Spec) ([]backend.File, error) {
	fixed := []backend.File{
		{Name: "Runtime.swift", Body: runtimeSwift},
		{Name: "Preview.swift", Body: previewSwift},
	}
	return emitFiles(s, fixed, emitDriver)
}

func emitDriver(s *spec.Spec, n structNames) string {
	b := &buf{}

	b.line("%s", header)
	b.line("import Foundation")
	b.line("")
	b.line("var frames: [Any] = []")
	b.line("for state in readStates() {")
	b.in()
	if len(s.Watches) == 0 {
		b.line("let results = Results()")
	} else {
		b.line("var results = Results()")
		for _, w := range s.Watches {
			// A watch a state leaves out is one that never answered, which is
			// the zero result and not a poll that succeeded with nothing.
			b.line("if let o = state.watches[%s] {", celswift.SwiftString(w.Name))
			b.in()
			b.line("results.%s = %s", decl(w.Name), sampleCall(w, n))
			b.out()
			b.line("}")
		}
	}
	b.line("frames.append(frame(state.name, renderFace(results), renderMenu(results)))")
	b.out()
	b.line("}")
	b.line("emit(frames)")
	return b.String()
}

func sampleCall(w spec.Watch, n structNames) string {
	switch w.Kind {
	case spec.WatchHTTP:
		return fmt.Sprintf("%s(HTTPOutcome(sample: o))", n.resultTypeName(w))
	case spec.WatchExists:
		return fmt.Sprintf("%s(exists: o.exists ?? false)", n.resultTypeName(w))
	default:
		return fmt.Sprintf("%s(RunOutcome(sample: o))", n.resultTypeName(w))
	}
}
