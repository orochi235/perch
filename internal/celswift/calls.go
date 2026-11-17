package celswift

import (
	"fmt"

	celast "cel.dev/cel-go/common/ast"
	"github.com/orochi235/perch/v2/internal/spec"
)

func (e *Env) lowerIndex(args []celast.Expr, src string) (value, error) {
	if len(args) != 2 {
		return value{}, fmt.Errorf("%q: indexing takes one index", src)
	}
	recv, err := e.lower(args[0], src)
	if err != nil {
		return value{}, err
	}
	idx, err := e.lower(args[1], src)
	if err != nil {
		return value{}, err
	}
	shown := recv.shown() + "[" + idx.shown() + "]"
	switch recv.typ.Kind {
	case spec.TypeList:
		var i string
		switch idx.typ.Kind {
		case spec.TypeInt:
			i = idx.swift
		case spec.TypeAny:
			i = idx.swift + ".asInt"
		default:
			return value{}, fmt.Errorf("%q: a list index must be an int, got %v", src, idx.typ.Kind)
		}
		// at() reads past the end as the element's zero value. A list is empty
		// until the first poll lands, so a subscript would trap at launch.
		return value{swift: recv.swift + ".at(" + i + ")", typ: recv.typ.Elem, display: shown}, nil
	case spec.TypeAny:
		switch idx.typ.Kind {
		case spec.TypeInt, spec.TypeString, spec.TypeAny:
		default:
			return value{}, fmt.Errorf("%q: an index must be an int or a string, got %v", src, idx.typ.Kind)
		}
		return value{swift: recv.swift + "[" + idx.swift + "]", typ: typ(spec.TypeAny), display: shown}, nil
	}
	return value{}, fmt.Errorf("%q: cannot index a %v", src, recv.typ.Kind)
}

func (e *Env) lowerIn(args []celast.Expr, src string) (value, error) {
	if len(args) != 2 {
		return value{}, fmt.Errorf("%q: in takes two operands", src)
	}
	elem, err := e.lower(args[0], src)
	if err != nil {
		return value{}, err
	}
	list, err := e.lower(args[1], src)
	if err != nil {
		return value{}, err
	}
	switch list.typ.Kind {
	case spec.TypeList:
		if !sameType(list.typ.Elem, elem.typ) {
			return value{}, fmt.Errorf("%q: in compares %s against a list of %s", src, describe(elem.typ), describe(list.typ.Elem))
		}
		return value{swift: list.swift + ".contains(" + elem.swift + ")", typ: typ(spec.TypeBool)}, nil
	case spec.TypeAny:
		switch elem.typ.Kind {
		case spec.TypeObject, spec.TypeList:
			return value{}, fmt.Errorf("%q: in against an untyped list looks for a single value, not a %v", src, elem.typ.Kind)
		}
		return value{swift: list.swift + ".contains(" + asJSON(elem) + ")", typ: typ(spec.TypeBool)}, nil
	}
	return value{}, fmt.Errorf("%q: in needs a list on the right, got %v", src, list.typ.Kind)
}

func (e *Env) lowerSize(c celast.CallExpr, args []celast.Expr, src string) (value, error) {
	target, err := e.soleOperand(c, args, "size", src)
	if err != nil {
		return value{}, err
	}
	switch target.typ.Kind {
	case spec.TypeList, spec.TypeString:
		return value{swift: target.swift + ".count", typ: typ(spec.TypeInt)}, nil
	case spec.TypeAny:
		return value{swift: target.swift + ".size", typ: typ(spec.TypeInt)}, nil
	}
	return value{}, fmt.Errorf("%q: size() needs a list or a string, got %v", src, target.typ.Kind)
}

func (e *Env) lowerStringCast(c celast.CallExpr, args []celast.Expr, src string) (value, error) {
	target, err := e.soleOperand(c, args, "string", src)
	if err != nil {
		return value{}, err
	}
	s, err := asString(target, src)
	if err != nil {
		return value{}, err
	}
	return value{swift: s, typ: typ(spec.TypeString)}, nil
}

func (e *Env) lowerStringMethod(name string, c celast.CallExpr, args []celast.Expr, src string) (value, error) {
	if !c.IsMemberFunction() || len(args) != 1 {
		return value{}, fmt.Errorf("%q: %s is written s.%s(t)", src, name, name)
	}
	recv, err := e.lower(c.Target(), src)
	if err != nil {
		return value{}, err
	}
	arg, err := e.lower(args[0], src)
	if err != nil {
		return value{}, err
	}
	swiftName := "contains"
	if name == "startsWith" {
		swiftName = "hasPrefix"
	}
	rs, err := asString(recv, src)
	if err != nil {
		return value{}, err
	}
	as, err := asString(arg, src)
	if err != nil {
		return value{}, err
	}
	return value{
		swift: fmt.Sprintf("%s.%s(%s)", rs, swiftName, as),
		typ:   typ(spec.TypeBool),
	}, nil
}

// soleOperand accepts both spellings CEL allows: f(x) and x.f().
func (e *Env) soleOperand(c celast.CallExpr, args []celast.Expr, name, src string) (value, error) {
	if c.IsMemberFunction() {
		if len(args) != 0 {
			return value{}, fmt.Errorf("%q: %s() takes no arguments", src, name)
		}
		return e.lower(c.Target(), src)
	}
	if len(args) != 1 {
		return value{}, fmt.Errorf("%q: %s() takes one argument", src, name)
	}
	return e.lower(args[0], src)
}
