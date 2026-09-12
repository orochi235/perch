// Package celswift lowers CEL expressions to Swift source at build time.
// Nothing evaluates CEL in the generated app: an expression perch cannot lower
// is a build error, not a widget that silently shows nothing.
package celswift

import "github.com/orochi235/perch/internal/spec"

// Env is the type environment an expression is lowered against: one binding per
// watch, plus `it` while inside an each:.
type Env struct {
	vars []binding
}

type binding struct {
	name  string
	typ   *spec.Type
	swift string // how the binding is spelled in generated Swift
	local bool   // a name the emitted function declares, not a field of the results
}

// NewEnv binds each watch to the fields its kind produces.
func NewEnv(watches []spec.Watch) *Env {
	e := &Env{}
	for _, w := range watches {
		e.vars = append(e.vars, binding{name: w.Name, typ: watchType(w), swift: w.Name})
	}
	return e
}

// WithEach returns a copy of e with `it` bound to elem, for lowering inside an
// each: item.
func (e *Env) WithEach(elem *spec.Type, swiftName string) *Env {
	out := &Env{vars: append([]binding(nil), e.vars...)}
	out.vars = append(out.vars, binding{name: "it", typ: elem, swift: swiftName, local: true})
	return out
}

// WithState returns a copy of e with name bound to a boolean the emitted code
// spells swiftName. Declaring states one at a time, in order, is what makes a
// condition naming a later state fail as an undeclared name rather than
// needing a cycle check of its own.
func (e *Env) WithState(name, swiftName string) *Env {
	out := &Env{vars: append([]binding(nil), e.vars...)}
	out.vars = append(out.vars, binding{
		name:  name,
		typ:   &spec.Type{Kind: spec.TypeBool},
		swift: swiftName,
	})
	return out
}

func (e *Env) lookup(name string) (binding, bool) {
	// Later bindings shadow earlier ones, so `it` wins inside an each:.
	for i := len(e.vars) - 1; i >= 0; i-- {
		if e.vars[i].name == name {
			return e.vars[i], true
		}
	}
	return binding{}, false
}

func (e *Env) names() []string {
	out := make([]string, 0, len(e.vars))
	for _, v := range e.vars {
		out = append(out, v.name)
	}
	return out
}

func obj(fields ...spec.Field) *spec.Type {
	return &spec.Type{Kind: spec.TypeObject, Fields: fields}
}

func field(name string, k spec.TypeKind) spec.Field {
	return spec.Field{Name: name, Type: &spec.Type{Kind: k}}
}

// watchType is the record a watch binds, per the design doc's table.
func watchType(w spec.Watch) *spec.Type {
	switch w.Kind {
	case spec.WatchExists:
		return obj(field("ok", spec.TypeBool))
	case spec.WatchLaunchAgent:
		t := obj(
			field("installed", spec.TypeBool),
			field("loaded", spec.TypeBool),
			field("running", spec.TypeBool),
			field("pid", spec.TypeInt),
			field("label", spec.TypeString),
			field("plist", spec.TypeString),
			field("domain", spec.TypeString),
			field("target", spec.TypeString),
		)
		t.Hints = map[string]string{
			"ok": "A LaunchAgent has two answers and they differ: .loaded is launchd knowing the label, .running is it having a process",
		}
		return t
	case spec.WatchHTTP:
		fs := []spec.Field{
			field("ok", spec.TypeBool),
			field("status", spec.TypeInt),
			field("out", spec.TypeString),
		}
		return obj(append(fs, dataField(w)...)...)
	default:
		fs := []spec.Field{
			field("ok", spec.TypeBool),
			field("code", spec.TypeInt),
			field("out", spec.TypeString),
			field("err", spec.TypeString),
		}
		return obj(append(fs, dataField(w)...)...)
	}
}

func dataField(w spec.Watch) []spec.Field {
	if !w.JSON {
		return nil
	}
	t := w.Shape
	if t == nil {
		t = &spec.Type{Kind: spec.TypeAny}
	}
	return []spec.Field{{Name: "data", Type: t}}
}

// Prefixed returns a copy of e whose watch bindings are spelled with prefix in
// generated Swift, so expressions reach them through the results record rather
// than through locals the emitted code might never use. `it` is a loop
// variable in scope already, so it is left alone.
func (e *Env) Prefixed(prefix string) *Env {
	out := &Env{vars: append([]binding(nil), e.vars...)}
	for i := range out.vars {
		if !out.vars[i].local {
			out.vars[i].swift = prefix + out.vars[i].swift
		}
	}
	return out
}
