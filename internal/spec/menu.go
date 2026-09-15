package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ActionKind is what activating a menu item does.
type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionRun
	ActionOpen
	ActionPost
	ActionQuit
	ActionAgent
	ActionSwift
	ActionWindow
)

func (k ActionKind) String() string {
	switch k {
	case ActionNone:
		return "none"
	case ActionRun:
		return "run"
	case ActionOpen:
		return "open"
	case ActionPost:
		return "post"
	case ActionQuit:
		return "quit"
	case ActionAgent:
		return "agent"
	case ActionSwift:
		return "swift"
	case ActionWindow:
		return "window"
	}
	return "unknown"
}

// AgentVerb is what an agent: action does to a LaunchAgent. The set is closed
// because each one is a launchctl invocation perch writes, not the author.
type AgentVerb string

const (
	AgentStart   AgentVerb = "start"
	AgentStop    AgentVerb = "stop"
	AgentRestart AgentVerb = "restart"
)

// AgentVerbs is every verb agent: accepts, in the order the docs list them.
var AgentVerbs = []AgentVerb{AgentStart, AgentStop, AgentRestart}

// WindowVerb is what a window: action does to the declared window.
type WindowVerb string

const (
	WindowOpen   WindowVerb = "open"
	WindowClose  WindowVerb = "close"
	WindowReload WindowVerb = "reload"
)

// WindowVerbs is every verb window: accepts, in the order the docs list them.
var WindowVerbs = []WindowVerb{WindowOpen, WindowClose, WindowReload}

// Action is the single verb an item carries. Run is argv and never a shell.
type Action struct {
	Kind     ActionKind
	Run      []string
	Open     string
	PostURL  string
	PostBody string // compact JSON, template holes preserved
	Agent    string // ActionAgent: the launchagent watch acted on
	Verb     AgentVerb
	Swift    string // ActionSwift: a dotted path, Type.method
	Window   WindowVerb
}

// Item is one menu entry: a separator, a label, or a label with an action,
// optionally repeated with each: and guarded with when:.
type Item struct {
	Separator bool
	Text      string
	When      string
	Each      string
	Menu      []Item
	Action    Action
	// Scope is the use this item came from, or "" for the file's own.
	Scope string

	outlet   string // set with isOutlet: the outlet's name, "" for the default
	isOutlet bool
	path     string // where the author wrote it, for errors after placement
}

type itemFields struct {
	Text   string    `yaml:"text"`
	When   string    `yaml:"when"`
	Each   string    `yaml:"each"`
	Menu   yaml.Node `yaml:"menu"`
	Run    []string  `yaml:"run"`
	Open   string    `yaml:"open"`
	Post   *rawPost  `yaml:"post"`
	Quit   bool      `yaml:"quit"`
	Agent  string    `yaml:"agent"`
	Swift  string    `yaml:"swift"`
	Window string    `yaml:"window"`
}

type rawPost struct {
	URL  string    `yaml:"url"`
	Body yaml.Node `yaml:"body"`
}

func parseMenu(n *yaml.Node, path string) ([]Item, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: want a list of menu items", path)
	}
	out := make([]Item, 0, len(n.Content))
	for i, c := range n.Content {
		it, err := parseItem(c, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func parseItem(n *yaml.Node, path string) (Item, error) {
	if name, ok, err := outletMark(n, path); err != nil {
		return Item{}, err
	} else if ok {
		return Item{outlet: name, isOutlet: true, path: path}, nil
	}
	if n.Kind == yaml.ScalarNode {
		if n.Value == "separator" {
			return Item{Separator: true, path: path}, nil
		}
		return Item{}, fmt.Errorf("%s: bare %q is not a menu item; only 'separator' and 'outlet' are", path, n.Value)
	}
	if n.Kind != yaml.MappingNode {
		return Item{}, fmt.Errorf("%s: want a menu item mapping", path)
	}

	var f itemFields
	if err := decodeStrict(n, &f, path); err != nil {
		return Item{}, err
	}

	sub, err := parseMenu(&f.Menu, path+".menu")
	if err != nil {
		return Item{}, err
	}
	act, err := actionFrom(f, path)
	if err != nil {
		return Item{}, err
	}
	// Refused here, not in validation: an outlet nothing fills empties the submenu.
	if len(sub) > 0 && act.Kind != ActionNone {
		return Item{}, fmt.Errorf("%s: has a submenu and a %s action; opening a submenu supersedes the action, so it would never run", path, act.Kind)
	}
	return Item{Text: f.Text, When: f.When, Each: f.Each, Menu: sub, Action: act, path: path}, nil
}

func actionFrom(f itemFields, path string) (Action, error) {
	var a Action
	var verbs []string
	if f.Run != nil {
		verbs = append(verbs, "run")
		a.Kind, a.Run = ActionRun, f.Run
	}
	if f.Open != "" {
		verbs = append(verbs, "open")
		a.Kind, a.Open = ActionOpen, f.Open
	}
	if f.Post != nil {
		verbs = append(verbs, "post")
		body, err := jsonFromNode(&f.Post.Body)
		if err != nil {
			return a, fmt.Errorf("%s.post.body: %w", path, err)
		}
		a.Kind, a.PostURL, a.PostBody = ActionPost, f.Post.URL, body
	}
	if f.Quit {
		verbs = append(verbs, "quit")
		a.Kind = ActionQuit
	}
	if f.Agent != "" {
		verbs = append(verbs, "agent")
		name, verb, err := parseAgentAction(f.Agent, path)
		if err != nil {
			return a, err
		}
		a.Kind, a.Agent, a.Verb = ActionAgent, name, verb
	}
	if f.Swift != "" {
		verbs = append(verbs, "swift")
		target, err := parseSwiftAction(f.Swift, path)
		if err != nil {
			return a, err
		}
		a.Kind, a.Swift = ActionSwift, target
	}
	if f.Window != "" {
		verbs = append(verbs, "window")
		verb, err := parseWindowAction(f.Window, path)
		if err != nil {
			return a, err
		}
		a.Kind, a.Window = ActionWindow, verb
	}
	if len(verbs) > 1 {
		return a, fmt.Errorf("%s: has %v; an item takes at most one of run, open, post, quit, agent, swift or window", path, verbs)
	}
	return a, nil
}

// parseAgentAction reads `<watch>.<verb>`. A watch name holds no dot, so the
// one separator is unambiguous.
func parseAgentAction(src, path string) (string, AgentVerb, error) {
	name, rest, found := strings.Cut(src, ".")
	if !found {
		return "", "", fmt.Errorf("%s.agent: %q names no verb; write <watch>.%s", path, src, strings.Join(verbList(), ", <watch>."))
	}
	for _, v := range AgentVerbs {
		if rest == string(v) {
			return name, v, nil
		}
	}
	return "", "", fmt.Errorf("%s.agent: %q is not something perch can do to a LaunchAgent; it does %s", path, rest, strings.Join(verbList(), ", "))
}

// swiftPath is Type.method: two Swift identifiers and one dot. Dotted rather
// than bare so a hand-written name cannot collide with an emitted one —
// Controller, Results, Draw, Watcher, renderFace, renderMenu.
var swiftPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\.[A-Za-z_][A-Za-z0-9_]*$`)

func parseSwiftAction(src, path string) (string, error) {
	if !swiftPath.MatchString(src) {
		return "", fmt.Errorf("%s.swift: %q is not <Type>.<method>; swift: names a static method in menubar/Sources/, e.g. Dash.showPreferences", path, src)
	}
	return src, nil
}

// parseWindowAction reads a bare verb. There is one window per app, so unlike
// agent: there is nothing to name.
func parseWindowAction(src, path string) (WindowVerb, error) {
	names := make([]string, 0, len(WindowVerbs))
	for _, v := range WindowVerbs {
		if src == string(v) {
			return v, nil
		}
		names = append(names, string(v))
	}
	return "", fmt.Errorf("%s.window: %q is not something perch can do to a window; it does %s", path, src, strings.Join(names, ", "))
}

func verbList() []string {
	out := make([]string, 0, len(AgentVerbs))
	for _, v := range AgentVerbs {
		out = append(out, string(v))
	}
	return out
}

func jsonFromNode(n *yaml.Node) (string, error) {
	if n == nil || n.Kind == 0 {
		return "", nil
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeStrict rejects keys the target struct does not declare, so a typo is a
// build error rather than a silently missing menu item. yaml.Node.Decode has no
// strict mode, hence the round trip through a Decoder.
func decodeStrict(n *yaml.Node, into any, path string) error {
	var buf bytes.Buffer
	if err := yaml.NewEncoder(&buf).Encode(n); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	dec := yaml.NewDecoder(&buf)
	dec.KnownFields(true)
	if err := dec.Decode(into); err != nil {
		if path == "" {
			return fmt.Errorf("%s", readable(err))
		}
		return fmt.Errorf("%s: %s", path, readable(err))
	}
	return nil
}

// yaml.v3 reports both of these by naming the Go type and counting lines in the
// re-encoded copy. Neither is something the author of a menubar.yaml can act on,
// so readable says the same thing in the document's own vocabulary.
var (
	unknownField = regexp.MustCompile("field (\\S+) not found in type \\S+")
	wrongType    = regexp.MustCompile("cannot unmarshal !!(\\w+)(?: `[^`]*`)? into (\\S+)")
)

var goTypes = map[string]string{
	"[]string": "a list of strings",
	"string":   "a string",
	"bool":     "true or false",
	"int":      "a number",
}

var yamlTags = map[string]string{
	"seq": "a list", "map": "a mapping", "str": "a string",
	"int": "a number", "float": "a number", "bool": "true or false", "null": "nothing",
}

func readable(err error) string {
	msg := err.Error()

	var keys []string
	for _, m := range unknownField.FindAllStringSubmatch(msg, -1) {
		keys = append(keys, strconv.Quote(m[1]))
	}
	if len(keys) == 1 {
		return "unknown key " + keys[0]
	}
	if len(keys) > 1 {
		return "unknown keys " + strings.Join(keys, ", ")
	}
	if m := wrongType.FindStringSubmatch(msg); m != nil {
		got := yamlTags[m[1]]
		if got == "" {
			got = m[1]
		}
		return fmt.Sprintf("want %s, got %s", want(m[2]), got)
	}
	return msg
}

// want names what one of the parser's own structs takes. Everything not in
// goTypes is a struct, so it takes a mapping of keys.
func want(goType string) string {
	if s, ok := goTypes[goType]; ok {
		return s
	}
	return "a mapping"
}
