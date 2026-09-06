// Package backend is the seam between a validated Spec and a platform.
// swift-appkit is the only implementation; others get a name and nothing else.
package backend

import "github.com/orochi235/perch/internal/spec"

// File is one emitted source file, written under menubar/Generated/.
type File struct {
	Name string
	Body []byte
}

// Backend turns a Spec into source for one platform.
type Backend interface {
	Name() string
	Emit(*spec.Spec) ([]File, error)
}
