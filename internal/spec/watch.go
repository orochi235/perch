package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// WatchKind is which of the three sources a watch polls.
type WatchKind int

const (
	WatchRun WatchKind = iota
	WatchHTTP
	WatchExists
	WatchLaunchAgent
)

func (k WatchKind) String() string {
	switch k {
	case WatchRun:
		return "run"
	case WatchHTTP:
		return "http"
	case WatchExists:
		return "exists"
	case WatchLaunchAgent:
		return "launchagent"
	}
	return "unknown"
}

// Watch is one polled source. Its results bind under its name in expressions.
type Watch struct {
	Name   string
	Kind   WatchKind
	Run    []string
	HTTP   string
	Exists string
	Label  string // WatchLaunchAgent: the launchd label
	Plist  string // WatchLaunchAgent: the LaunchAgents file, defaulted from Label
	JSON   bool
	Shape  *Type
}

type rawWatch struct {
	Run         []string  `yaml:"run"`
	HTTP        string    `yaml:"http"`
	Exists      string    `yaml:"exists"`
	LaunchAgent string    `yaml:"launchagent"`
	Plist       string    `yaml:"plist"`
	JSON        bool      `yaml:"json"`
	Shape       yaml.Node `yaml:"shape"`
}

func parseWatches(n *yaml.Node) ([]Watch, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("watch: want a mapping of names to watches")
	}
	var out []Watch
	for i := 0; i+1 < len(n.Content); i += 2 {
		name := n.Content[i].Value
		var raw rawWatch
		if err := decodeStrict(n.Content[i+1], &raw, "watch."+name); err != nil {
			return nil, err
		}
		w, err := watchFromRaw(name, raw)
		if err != nil {
			return nil, err
		}
		if raw.Shape.Kind != 0 {
			t, err := parseType(&raw.Shape, "watch."+name+".shape")
			if err != nil {
				return nil, err
			}
			w.Shape = t
		}
		out = append(out, w)
	}
	return out, nil
}

func watchFromRaw(name string, raw rawWatch) (Watch, error) {
	w := Watch{Name: name, JSON: raw.JSON}
	var kinds []string
	if raw.Run != nil {
		kinds = append(kinds, "run")
		w.Kind, w.Run = WatchRun, raw.Run
	}
	if raw.HTTP != "" {
		kinds = append(kinds, "http")
		w.Kind, w.HTTP = WatchHTTP, raw.HTTP
	}
	if raw.Exists != "" {
		kinds = append(kinds, "exists")
		w.Kind, w.Exists = WatchExists, raw.Exists
	}
	if raw.LaunchAgent != "" {
		kinds = append(kinds, "launchagent")
		w.Kind, w.Label = WatchLaunchAgent, raw.LaunchAgent
		w.Plist = raw.Plist
		if w.Plist == "" {
			w.Plist = "~/Library/LaunchAgents/" + raw.LaunchAgent + ".plist"
		}
	}
	if raw.Plist != "" && raw.LaunchAgent == "" {
		return w, fmt.Errorf("watch.%s: plist says where a launchagent watch reads its file, so it needs launchagent:", name)
	}
	switch len(kinds) {
	case 0:
		return w, fmt.Errorf("watch.%s: needs one of run, http, exists or launchagent", name)
	case 1:
		return w, nil
	default:
		return w, fmt.Errorf("watch.%s: has %v; a watch takes exactly one of run, http, exists or launchagent", name, kinds)
	}
}
