package celswift

import (
	"fmt"

	celast "cel.dev/cel-go/common/ast"
	"github.com/orochi235/perch/v2/internal/spec"
)

func (e *Env) lowerComparison(op string, l, r celast.Expr, src string) (value, error) {
	lv, err := e.lower(l, src)
	if err != nil {
		return value{}, err
	}
	rv, err := e.lower(r, src)
	if err != nil {
		return value{}, err
	}
	ls, rs, err := coercePair(lv, rv, src)
	if err != nil {
		return value{}, err
	}
	if op == "==" || op == "!=" {
		return value{swift: "(" + ls + " " + op + " " + rs + ")", typ: typ(spec.TypeBool)}, nil
	}
	switch {
	case lv.typ.Kind == spec.TypeBool || rv.typ.Kind == spec.TypeBool:
		return value{}, fmt.Errorf("%q: %s orders numbers and strings, not bools", src, op)
	case lv.typ.Kind == spec.TypeAny && rv.typ.Kind == spec.TypeAny:
		// JSONValue is not Comparable: two untyped values order only when both
		// are numbers or both are strings, and any other pair is false.
		return value{swift: "(" + ls + ".order(" + rs + ").map { $0 " + op + " 0 } ?? false)", typ: typ(spec.TypeBool)}, nil
	}
	return value{swift: "(" + ls + " " + op + " " + rs + ")", typ: typ(spec.TypeBool)}, nil
}

// sameType is whether two values have one Swift type. Each declared object is
// its own struct, so two objects match only when they are the same declaration.
func sameType(a, b *spec.Type) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case spec.TypeList:
		return sameType(a.Elem, b.Elem)
	case spec.TypeObject:
		return a == b
	}
	return true
}

// describe names a type for a diagnostic, down to a list's elements.
func describe(t *spec.Type) string {
	switch t.Kind {
	case spec.TypeList:
		return "a list of " + bare(t.Elem)
	case spec.TypeInt, spec.TypeAny:
		return "an " + bare(t)
	case spec.TypeObject:
		return "an " + bare(t)
	}
	return "a " + bare(t)
}

func bare(t *spec.Type) string {
	switch t.Kind {
	case spec.TypeList:
		return "list of " + bare(t.Elem)
	case spec.TypeObject:
		return "object {" + fieldNames(t) + "}"
	}
	return t.Kind.String()
}

// coercePair brings two operands to one Swift type. A dynamic operand meeting a
// concrete one takes the concrete one's type; two dynamic operands compare as
// JSONValue.
func coercePair(l, r value, src string) (string, string, error) {
	switch {
	case l.typ.Kind == r.typ.Kind && l.typ.Kind != spec.TypeObject && l.typ.Kind != spec.TypeList:
		return l.swift, r.swift, nil
	case l.typ.Kind == spec.TypeAny && r.typ.Kind == spec.TypeAny:
		return l.swift, r.swift, nil
	case l.typ.Kind == spec.TypeAny:
		s, err := coerceTo(l, r.typ.Kind, src)
		return s, r.swift, err
	case r.typ.Kind == spec.TypeAny:
		s, err := coerceTo(r, l.typ.Kind, src)
		return l.swift, s, err
	}
	return "", "", fmt.Errorf("%q: cannot compare a %v with a %v", src, l.typ.Kind, r.typ.Kind)
}

func coerceTo(v value, k spec.TypeKind, src string) (string, error) {
	switch k {
	case spec.TypeBool:
		return v.swift + ".asBool", nil
	case spec.TypeInt:
		return v.swift + ".asInt", nil
	case spec.TypeDouble:
		return v.swift + ".asDouble", nil
	case spec.TypeString:
		return v.swift + ".asString", nil
	}
	return "", fmt.Errorf("%q: cannot compare against a %v", src, k)
}

func asBool(v value, src string) (string, error) {
	switch v.typ.Kind {
	case spec.TypeBool:
		return v.swift, nil
	case spec.TypeAny:
		return v.swift + ".asBool", nil
	}
	return "", fmt.Errorf("%q: want a bool, got %v", src, v.typ.Kind)
}

// asString refuses the two kinds with no text form. Lowering them anyway would
// emit Swift that does not compile, which reports the mistake against generated
// source instead of against the expression that caused it.
func asString(v value, src string) (string, error) {
	switch v.typ.Kind {
	case spec.TypeString:
		return v.swift, nil
	case spec.TypeAny:
		return v.swift + ".asString", nil
	case spec.TypeObject, spec.TypeList:
		return "", fmt.Errorf("%q: a %v has no text form; select a field of it", src, v.typ.Kind)
	}
	return "String(" + v.swift + ")", nil
}

func asJSON(v value) string {
	if v.typ.Kind == spec.TypeAny {
		return v.swift
	}
	return "JSONValue(" + v.swift + ")"
}
