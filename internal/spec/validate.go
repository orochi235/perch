package spec

import (
	"fmt"
	"strings"
)

func (s *Spec) validate() error {
	if s.App.Name == "" {
		return fmt.Errorf("app.name: required")
	}
	if err := checkAppName(s.App.Name); err != nil {
		return err
	}
	if s.App.ID == "" {
		return fmt.Errorf("app.id: required (the bundle identifier, e.g. dev.example.menubar)")
	}
	if err := checkAppID(s.App.ID); err != nil {
		return err
	}
	if s.App.Icon.IsZero() {
		return fmt.Errorf("app.icon: required (an SF Symbol name, or {asset: <name>} for a file in menubar/Icons)")
	}
	if s.App.Interval <= 0 {
		return fmt.Errorf("app.interval: required and must be positive, got %v", s.App.Interval)
	}
	if s.Window != nil {
		if err := s.Window.validate(); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, w := range s.Watches {
		if seen[w.Name] {
			return fmt.Errorf("watch.%s: declared twice", w.Name)
		}
		seen[w.Name] = true
		if err := w.validate(); err != nil {
			return err
		}
	}
	for _, u := range s.Uses {
		if seen[u.Name] {
			return fmt.Errorf("use.%s: %q is already a watch; an expression names watches and uses the same way, so it could not tell them apart", u.Name, u.Name)
		}
		where := u.where() + ": "
		for _, w := range u.Watches {
			if err := w.validate(); err != nil {
				return fmt.Errorf("%s%w", where, err)
			}
		}
		if err := checkStates(u.States, u.Watches, nil, where); err != nil {
			return err
		}
	}
	if err := s.validateStates(); err != nil {
		return err
	}
	for i, r := range s.Status {
		if r.When == "" && i != len(s.Status)-1 {
			return fmt.Errorf("%s: a rule with no when: always matches, so it must be last; %d rule(s) after it can never apply", r.path, len(s.Status)-1-i)
		}
	}
	if err := s.validateQuit(); err != nil {
		return err
	}
	return validateItems(s.Menu, s.Watches, s.Window)
}

func validateItems(items []Item, watches []Watch, window *Window) error {
	for _, it := range items {
		if err := it.Action.validate(it.path, watches, window); err != nil {
			return err
		}
		if err := validateItems(it.Menu, watches, window); err != nil {
			return err
		}
	}
	return nil
}

func (a Action) validate(path string, watches []Watch, window *Window) error {
	switch a.Kind {
	case ActionRun:
		if len(a.Run) == 0 {
			return fmt.Errorf("%s.run: empty; run takes the argv of a command", path)
		}
	case ActionPost:
		if a.PostURL == "" {
			return fmt.Errorf("%s.post: needs a url", path)
		}
	case ActionAgent:
		return checkAgentTarget(path, a.Agent, watches)
	case ActionWindow:
		if window == nil {
			return fmt.Errorf("%s.window: this file declares no window: block, so there is nothing to %s", path, a.Window)
		}
	}
	return nil
}

// checkAgentTarget refuses an agent: naming anything but a launchagent watch.
// The label and the plist path both come from that watch, so there is nothing
// to act on without one.
func checkAgentTarget(path, name string, watches []Watch) error {
	var agents []string
	for _, w := range watches {
		if w.Kind == WatchLaunchAgent {
			agents = append(agents, w.Name)
		}
		if w.Name != name {
			continue
		}
		if w.Kind != WatchLaunchAgent {
			return fmt.Errorf("%s.agent: %q is a %s watch; agent: acts on a launchagent watch, which is where the label and the plist come from", path, name, w.Kind)
		}
		return nil
	}
	if len(agents) == 0 {
		return fmt.Errorf("%s.agent: no watch named %q, and this file declares no launchagent watch", path, name)
	}
	return fmt.Errorf("%s.agent: no watch named %q; the launchagent watches are %s", path, name, strings.Join(agents, ", "))
}

func (w Watch) validate() error {
	path := "watch." + w.Name
	if err := checkName("watch", path, w.Name); err != nil {
		return err
	}
	if err := checkBound(path, w.Name, "watch"); err != nil {
		return err
	}
	if w.Kind == WatchRun && len(w.Run) == 0 {
		return fmt.Errorf("%s.run: empty; run takes the argv of a command", path)
	}
	if w.Kind == WatchExists && w.JSON {
		return fmt.Errorf("%s: json has no meaning on an exists watch, which only binds .ok", path)
	}
	if w.Kind == WatchLaunchAgent {
		if w.JSON {
			return fmt.Errorf("%s: json has no meaning on a launchagent watch; it binds the fields launchd reports, not decoded output", path)
		}
		if err := checkLabel(path, w.Label); err != nil {
			return err
		}
	}
	if w.Shape != nil && !w.JSON {
		return fmt.Errorf("%s: shape describes decoded JSON, so it needs json: true", path)
	}
	if w.Shape != nil {
		if err := validateType(w.Shape, path+".shape"); err != nil {
			return err
		}
	}
	return nil
}

// validateType checks that every field a shape declares can be both selected in
// an expression and declared in the emitted source.
func validateType(t *Type, path string) error {
	switch t.Kind {
	case TypeList:
		return validateType(t.Elem, path+"[]")
	case TypeObject:
		seen := map[string]bool{}
		for _, f := range t.Fields {
			if err := checkName("field", path, f.Name); err != nil {
				return err
			}
			if seen[f.Name] {
				return fmt.Errorf("%s: field %q declared twice", path, f.Name)
			}
			seen[f.Name] = true
			if err := validateType(f.Type, path+"."+f.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
