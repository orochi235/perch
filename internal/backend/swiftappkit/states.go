package swiftappkit

import (
	"fmt"
	"strings"

	"github.com/orochi235/perch/internal/celswift"
	"github.com/orochi235/perch/internal/spec"
)

// emitStates writes each of the file's states as a property of Results. A
// condition is lowered against the watches and uses, never these states: the
// ordering already excludes every earlier state, so naming one could only ever
// be a constant, and a guard that contributes nothing is worse than one
// refused. They are properties rather than locals because a state a given
// function never reads is then simply unread, instead of an unused binding the
// compiler warns about.
func emitStates(b *buf, s *spec.Spec) error {
	// self., because a watch or use named default reads as a keyword bare.
	e := celswift.NewEnv(s.Watches).WithUses(s.Uses).Prefixed("self.")
	return emitStateList(b, "Results", s.States, e, stateProp, "")
}

// emitUseStates writes each use's states as properties of its own struct, so
// a use's list is ordered against itself and nothing else.
func emitUseStates(b *buf, s *spec.Spec, n structNames) error {
	for _, u := range s.Uses {
		e := celswift.ForUseStates(u, "self")
		own := func(name string) string { return name }
		if err := emitStateList(b, n.useTypeName(u), u.States, e, own, "use."+u.Name+": "); err != nil {
			return err
		}
	}
	return nil
}

// emitStateList writes one ordered list as properties of typeName, each
// excluding every state before it. prop names a state's property, and where
// prefixes an error.
func emitStateList(b *buf, typeName string, states []spec.State, e *celswift.Env, prop func(string) string, where string) error {
	if len(states) == 0 {
		return nil
	}
	b.line("extension %s {", typeName)
	b.in()
	for i, st := range states {
		parts := make([]string, 0, i+1)
		for _, earlier := range states[:i] {
			parts = append(parts, "!self."+prop(earlier.Name))
		}
		if st.Cond == "" {
			if len(parts) == 0 {
				parts = append(parts, "true")
			}
		} else {
			cond, err := e.LowerCondition(st.Cond)
			if err != nil {
				return fmt.Errorf("%sstate[%d].%s: %w", where, i, st.Name, err)
			}
			// Parenthesized: a condition of a && b would otherwise come apart
			// under the leading negations, and still compile.
			parts = append(parts, "("+cond+")")
		}
		b.line("var %s: Bool { %s }", decl(prop(st.Name)), strings.Join(parts, " && "))
	}
	b.out()
	b.line("}")
	b.line("")
	return nil
}

// stateProp names a file state's property on Results. The prefix keeps it
// clear of the names the emitter mints; a watch or use named state_x beside a
// state x would still take the same name, which spec refuses.
func stateProp(name string) string { return "state_" + name }
