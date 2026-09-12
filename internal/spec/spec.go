// Package spec parses and validates menubar.yaml into the Spec that backends emit from.
package spec

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Spec is the validated contents of a menubar.yaml. It is perch's intermediate
// representation: backends emit from this and nothing else.
type Spec struct {
	App     App
	Watches []Watch
	States  []State
	Status  []StatusRule
	Menu    []Item
}

// App is the identity and cadence of the generated status-bar app.
type App struct {
	Name     string
	ID       string
	Icon     Icon
	Interval time.Duration
	// Sign is a keychain code signing identity. Empty signs the bundle ad-hoc,
	// which seals it but gives it a new identity on every build.
	Sign string
}

type rawSpec struct {
	App    rawApp    `yaml:"app"`
	Watch  yaml.Node `yaml:"watch"`
	State  yaml.Node `yaml:"state"`
	Status yaml.Node `yaml:"status"`
	Menu   yaml.Node `yaml:"menu"`
}

type rawApp struct {
	Name     string    `yaml:"name"`
	ID       string    `yaml:"id"`
	Icon     yaml.Node `yaml:"icon"`
	Interval string    `yaml:"interval"`
	Sign     string    `yaml:"sign"`
}

// Parse reads a menubar.yaml document into a Spec.
func Parse(src []byte) (*Spec, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	root, err := resolveAliases(doc.Content[0])
	if err != nil {
		return nil, err
	}
	var raw rawSpec
	// No path: project.Load has already named the file.
	if err := decodeStrict(root, &raw, ""); err != nil {
		return nil, err
	}
	appIcon, err := parseIcon(&raw.App.Icon, "app.icon")
	if err != nil {
		return nil, err
	}
	s := &Spec{App: App{
		Name: raw.App.Name,
		ID:   raw.App.ID,
		Icon: appIcon,
		Sign: raw.App.Sign,
	}}
	if raw.App.Interval != "" {
		d, err := time.ParseDuration(raw.App.Interval)
		if err != nil {
			return nil, fmt.Errorf("app.interval: %w", err)
		}
		s.App.Interval = d
	}
	ws, err := parseWatches(&raw.Watch)
	if err != nil {
		return nil, err
	}
	s.Watches = ws
	states, err := parseStates(&raw.State)
	if err != nil {
		return nil, err
	}
	s.States = states
	st, err := parseStatus(&raw.Status)
	if err != nil {
		return nil, err
	}
	s.Status = st
	menu, err := parseMenu(&raw.Menu, "menu")
	if err != nil {
		return nil, err
	}
	s.Menu = menu
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// aliasBudget bounds the tree an expansion may produce. Nested anchors that
// each refer to the one above double at every level, so a short document can
// name a tree too large to hold.
const aliasBudget = 100000

// resolveAliases returns a copy of the document with every alias replaced by
// what it points at. The parser re-encodes subtrees on their own to reject
// unknown keys, and an anchor defined in a sibling subtree is not in the copy
// it encodes — which is what an author sharing one shape between two watches
// writes.
func resolveAliases(n *yaml.Node) (*yaml.Node, error) {
	r := &aliasResolver{path: map[*yaml.Node]bool{}, left: aliasBudget}
	return r.resolve(n)
}

type aliasResolver struct {
	path map[*yaml.Node]bool // anchors being expanded, so a cycle is not followed twice
	left int
}

func (r *aliasResolver) resolve(n *yaml.Node) (*yaml.Node, error) {
	if n == nil {
		return nil, nil
	}
	if r.left--; r.left < 0 {
		return nil, fmt.Errorf("this document's aliases expand to more than %d nodes", aliasBudget)
	}
	if n.Kind == yaml.AliasNode {
		if n.Alias == nil {
			return nil, fmt.Errorf("the alias *%s refers to nothing", n.Value)
		}
		if r.path[n.Alias] {
			return nil, fmt.Errorf("the alias *%s refers to itself", n.Value)
		}
		r.path[n.Alias] = true
		defer delete(r.path, n.Alias)
		return r.resolve(n.Alias)
	}
	if len(n.Content) == 0 {
		return n, nil
	}
	out := *n
	out.Anchor = ""
	out.Content = make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		sub, err := r.resolve(c)
		if err != nil {
			return nil, err
		}
		out.Content[i] = sub
	}
	return &out, nil
}
