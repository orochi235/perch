package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// TypeKind is one of the seven things a shape declaration can say.
type TypeKind int

const (
	TypeString TypeKind = iota
	TypeInt
	TypeDouble
	TypeBool
	TypeAny
	TypeObject
	TypeList
)

func (k TypeKind) String() string {
	switch k {
	case TypeString:
		return "string"
	case TypeInt:
		return "int"
	case TypeDouble:
		return "double"
	case TypeBool:
		return "bool"
	case TypeAny:
		return "any"
	case TypeObject:
		return "object"
	case TypeList:
		return "list"
	}
	return "unknown"
}

// Type is a declared watch shape: what a command prints, and nothing else.
type Type struct {
	Kind   TypeKind
	Fields []Field           // TypeObject, in document order
	Elem   *Type             // TypeList
	Hints  map[string]string // field name -> why this object does not have it
}

// Field is one named member of an object type.
type Field struct {
	Name string
	Type *Type
}

var scalars = map[string]TypeKind{
	"string": TypeString,
	"int":    TypeInt,
	"double": TypeDouble,
	"bool":   TypeBool,
	"any":    TypeAny,
}

func parseType(n *yaml.Node, path string) (*Type, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		k, ok := scalars[n.Value]
		if !ok {
			return nil, fmt.Errorf("%s: unknown type %q; shapes use string, int, double, bool, any, objects and [T]", path, n.Value)
		}
		return &Type{Kind: k}, nil

	case yaml.SequenceNode:
		if len(n.Content) != 1 {
			return nil, fmt.Errorf("%s: a list type is written [T] with exactly one element, got %d", path, len(n.Content))
		}
		elem, err := parseType(n.Content[0], path+"[]")
		if err != nil {
			return nil, err
		}
		return &Type{Kind: TypeList, Elem: elem}, nil

	case yaml.MappingNode:
		t := &Type{Kind: TypeObject}
		for i := 0; i+1 < len(n.Content); i += 2 {
			name := n.Content[i].Value
			ft, err := parseType(n.Content[i+1], path+"."+name)
			if err != nil {
				return nil, err
			}
			t.Fields = append(t.Fields, Field{Name: name, Type: ft})
		}
		return t, nil

	case yaml.AliasNode:
		return parseType(n.Alias, path)
	}
	return nil, fmt.Errorf("%s: cannot read as a type", path)
}
