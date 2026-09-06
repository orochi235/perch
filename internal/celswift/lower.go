package celswift

import (
	"fmt"
	"strconv"
	"strings"

	"cel.dev/cel-go/cel"
	celast "cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"github.com/orochi235/perch/internal/spec"
)

// value is a lowered subexpression: Swift source, the type it has there, and
// the way the author spelled it. Diagnostics quote display, never swift — the
// emitted spelling carries a results prefix and generated loop variables that
// mean nothing to whoever wrote the YAML.
type value struct {
	swift   string
	typ     *spec.Type
	display string
}

// shown is the spelling to put in a diagnostic.
func (v value) shown() string {
	if v.display != "" {
		return v.display
	}
	return v.swift
}

func typ(k spec.TypeKind) *spec.Type { return &spec.Type{Kind: k} }

var parser = newParser()

func newParser() *cel.Env {
	e, err := cel.NewEnv()
	if err != nil {
		panic("celswift: cel.NewEnv: " + err.Error())
	}
	return e
}

// LowerExpr lowers src to a Swift expression, keeping perch's own idea of its
// type. It is the entry point the boundary lowerings share.
func (e *Env) LowerExpr(src string) (string, error) {
	v, err := e.lowerString(src)
	if err != nil {
		return "", err
	}
	return v.swift, nil
}

// LowerCondition lowers src to a Swift Bool expression.
func (e *Env) LowerCondition(src string) (string, error) {
	v, err := e.lowerString(src)
	if err != nil {
		return "", err
	}
	return asBool(v, src)
}

// LowerText lowers src to a Swift String expression.
func (e *Env) LowerText(src string) (string, error) {
	v, err := e.lowerString(src)
	if err != nil {
		return "", err
	}
	return asString(v, src)
}

// LowerList lowers src to a Swift sequence expression and reports its element
// type, for each: items.
func (e *Env) LowerList(src string) (string, *spec.Type, error) {
	v, err := e.lowerString(src)
	if err != nil {
		return "", nil, err
	}
	switch v.typ.Kind {
	case spec.TypeList:
		return v.swift, v.typ.Elem, nil
	case spec.TypeAny:
		return v.swift + ".asArray", typ(spec.TypeAny), nil
	}
	return "", nil, fmt.Errorf("%q: each: needs a list, this is %v", src, v.typ.Kind)
}

func (e *Env) lowerString(src string) (value, error) {
	parsed, iss := parser.Parse(src)
	if iss != nil && iss.Err() != nil {
		return value{}, fmt.Errorf("%q: %w", src, iss.Err())
	}
	return e.lower(parsed.NativeRep().Expr(), src)
}

func (e *Env) lower(x celast.Expr, src string) (value, error) {
	switch x.Kind() {
	case celast.IdentKind:
		name := x.AsIdent()
		b, ok := e.lookup(name)
		if !ok {
			if len(e.vars) == 0 {
				return value{}, fmt.Errorf("%q: unknown name %q; no watches are declared", src, name)
			}
			return value{}, fmt.Errorf("%q: unknown name %q; bound here: %s", src, name, strings.Join(e.names(), ", "))
		}
		return value{swift: b.swift, typ: b.typ, display: b.name}, nil

	case celast.LiteralKind:
		return literal(x.AsLiteral(), src)

	case celast.SelectKind:
		return e.lowerSelect(x.AsSelect(), src)

	case celast.CallKind:
		return e.lowerCall(x.AsCall(), src)

	case celast.ComprehensionKind:
		return value{}, fmt.Errorf("%q: perch does not implement comprehensions (map, filter, all, exists)", src)

	case celast.ListKind:
		return value{}, fmt.Errorf("%q: perch does not implement list literals", src)

	case celast.MapKind, celast.StructKind:
		return value{}, fmt.Errorf("%q: perch does not implement map or struct literals", src)
	}
	return value{}, fmt.Errorf("%q: perch does not implement this expression", src)
}

func literal(v any, src string) (value, error) {
	switch l := v.(type) {
	case types.Int:
		return value{swift: strconv.FormatInt(int64(l), 10), typ: typ(spec.TypeInt)}, nil
	case types.Uint:
		return value{swift: strconv.FormatUint(uint64(l), 10), typ: typ(spec.TypeInt)}, nil
	case types.Double:
		return value{swift: strconv.FormatFloat(float64(l), 'g', -1, 64), typ: typ(spec.TypeDouble)}, nil
	case types.String:
		return value{swift: SwiftString(string(l)), typ: typ(spec.TypeString)}, nil
	case types.Bool:
		return value{swift: strconv.FormatBool(bool(l)), typ: typ(spec.TypeBool)}, nil
	}
	return value{}, fmt.Errorf("%q: perch does not implement this literal", src)
}

func (e *Env) lowerSelect(s celast.SelectExpr, src string) (value, error) {
	recv, err := e.lower(s.Operand(), src)
	if err != nil {
		return value{}, err
	}
	name := s.FieldName()
	switch recv.typ.Kind {
	case spec.TypeObject:
		for _, f := range recv.typ.Fields {
			if f.Name == name {
				if s.IsTestOnly() {
					// A declared shape always has the field, so has() is constant.
					return value{swift: "true", typ: typ(spec.TypeBool)}, nil
				}
				return value{swift: recv.swift + "." + name, typ: f.Type, display: recv.shown() + "." + name}, nil
			}
		}
		return value{}, fmt.Errorf("%q: %s has no field %q; it has %s", src, recv.shown(), name, fieldNames(recv.typ))
	case spec.TypeAny:
		acc := fmt.Sprintf("%s[%q]", recv.swift, name)
		shown := recv.shown() + "." + name
		if s.IsTestOnly() {
			return value{swift: acc + ".exists", typ: typ(spec.TypeBool), display: shown}, nil
		}
		return value{swift: acc, typ: typ(spec.TypeAny), display: shown}, nil
	default:
		return value{}, fmt.Errorf("%q: cannot select %q from a %v", src, name, recv.typ.Kind)
	}
}

var comparisons = map[string]string{
	"_==_": "==", "_!=_": "!=",
	"_<_": "<", "_<=_": "<=", "_>_": ">", "_>=_": ">=",
}

func (e *Env) lowerCall(c celast.CallExpr, src string) (value, error) {
	name := c.FunctionName()
	args := c.Args()

	if op, ok := comparisons[name]; ok && len(args) == 2 {
		return e.lowerComparison(op, args[0], args[1], src)
	}

	switch name {
	case "!_":
		return e.lowerNot(args, src)
	case "_&&_", "_||_":
		return e.lowerLogical(name, args, src)
	case "_?_:_":
		return e.lowerTernary(args, src)
	case "_[_]":
		return e.lowerIndex(args, src)
	case "@in":
		return e.lowerIn(args, src)
	case "size":
		return e.lowerSize(c, args, src)
	case "string":
		return e.lowerStringCast(c, args, src)
	case "startsWith", "contains":
		return e.lowerStringMethod(name, c, args, src)
	}
	return value{}, fmt.Errorf("%q: perch does not implement %s", src, displayName(name))
}

func (e *Env) lowerNot(args []celast.Expr, src string) (value, error) {
	if len(args) != 1 {
		return value{}, fmt.Errorf("%q: ! takes one operand", src)
	}
	a, err := e.lower(args[0], src)
	if err != nil {
		return value{}, err
	}
	s, err := asBool(a, src)
	if err != nil {
		return value{}, err
	}
	return value{swift: "!(" + s + ")", typ: typ(spec.TypeBool)}, nil
}

func (e *Env) lowerLogical(name string, args []celast.Expr, src string) (value, error) {
	op := "&&"
	if name == "_||_" {
		op = "||"
	}
	if len(args) != 2 {
		return value{}, fmt.Errorf("%q: %s takes two operands", src, op)
	}
	var parts []string
	for _, a := range args {
		v, err := e.lower(a, src)
		if err != nil {
			return value{}, err
		}
		s, err := asBool(v, src)
		if err != nil {
			return value{}, err
		}
		parts = append(parts, s)
	}
	return value{swift: "(" + parts[0] + " " + op + " " + parts[1] + ")", typ: typ(spec.TypeBool)}, nil
}

func (e *Env) lowerTernary(args []celast.Expr, src string) (value, error) {
	if len(args) != 3 {
		return value{}, fmt.Errorf("%q: ?: takes three operands", src)
	}
	cond, err := e.lower(args[0], src)
	if err != nil {
		return value{}, err
	}
	cs, err := asBool(cond, src)
	if err != nil {
		return value{}, err
	}
	t, err := e.lower(args[1], src)
	if err != nil {
		return value{}, err
	}
	f, err := e.lower(args[2], src)
	if err != nil {
		return value{}, err
	}
	if t.typ.Kind != f.typ.Kind {
		return value{}, fmt.Errorf("%q: both arms of ?: must have the same type, got %v and %v", src, t.typ.Kind, f.typ.Kind)
	}
	return value{swift: "(" + cs + " ? " + t.swift + " : " + f.swift + ")", typ: t.typ}, nil
}

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
	switch recv.typ.Kind {
	case spec.TypeList:
		if idx.typ.Kind != spec.TypeInt {
			return value{}, fmt.Errorf("%q: a list index must be an int, got %v", src, idx.typ.Kind)
		}
		return value{swift: recv.swift + "[" + idx.swift + "]", typ: recv.typ.Elem, display: recv.shown() + "[" + idx.shown() + "]"}, nil
	case spec.TypeAny:
		return value{swift: recv.swift + "[" + idx.swift + "]", typ: typ(spec.TypeAny), display: recv.shown() + "[" + idx.shown() + "]"}, nil
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
		if list.typ.Elem.Kind != elem.typ.Kind {
			return value{}, fmt.Errorf("%q: in compares a %v against a list of %v", src, elem.typ.Kind, list.typ.Elem.Kind)
		}
		return value{swift: list.swift + ".contains(" + elem.swift + ")", typ: typ(spec.TypeBool)}, nil
	case spec.TypeAny:
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
	return value{swift: "(" + ls + " " + op + " " + rs + ")", typ: typ(spec.TypeBool)}, nil
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

func fieldNames(t *spec.Type) string {
	out := make([]string, 0, len(t.Fields))
	for _, f := range t.Fields {
		out = append(out, f.Name)
	}
	return strings.Join(out, ", ")
}

// displayName turns CEL's internal operator spelling back into what an author wrote.
func displayName(fn string) string {
	if s, ok := strings.CutSuffix(strings.TrimPrefix(fn, "_"), "_"); ok {
		return s
	}
	return fn + "()"
}

// SwiftString renders s as a Swift string literal.
func SwiftString(s string) string { return `"` + escapeInterpolated(s) + `"` }
