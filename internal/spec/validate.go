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
	return s.validateItems(s.Menu)
}

// validateItems reports each item at it.path, where the author wrote it, so a
// template's items already name their use and file.
func (s *Spec) validateItems(items []Item) error {
	for _, it := range items {
		if err := s.validateAction(it.Action, it.path, it.Scope); err != nil {
			return err
		}
		if err := s.validateItems(it.Menu); err != nil {
			return err
		}
	}
	return nil
}

func (s *Spec) validateAction(a Action, path, scope string) error {
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
		if _, err := s.AgentWatch(scope, a.Agent); err != nil {
			return fmt.Errorf("%s.agent: %w", path, err)
		}
	case ActionWindow:
		if s.Window == nil {
			return fmt.Errorf("%s.window: this file declares no window: block, so there is nothing to %s", path, a.Window)
		}
	}
	return nil
}

// AgentWatch finds the launchagent watch an agent: target names, as seen from
// an item in scope: "" for the file's own, or the use a template's item came
// from. A target is <watch>, <use>.<watch>, or self.<watch> inside a template.
func (s *Spec) AgentWatch(scope, target string) (Watch, error) {
	parts := strings.Split(target, ".")
	switch len(parts) {
	case 1:
		if scope != "" {
			return Watch{}, fmt.Errorf("%q: a template sees only self, so name its watch as self.%s", target, target)
		}
		return launchAgentIn(parts[0], s.Watches, "this file")
	case 2:
		useName := parts[0]
		switch {
		case useName == "self" && scope == "":
			return Watch{}, fmt.Errorf("%q: self is bound only inside a template, where it is that use", target)
		case useName == "self":
			useName = scope
		case scope != "":
			return Watch{}, fmt.Errorf("%q: a template sees only self, so write self.%s", target, parts[1])
		}
		for _, u := range s.Uses {
			if u.Name == useName {
				return launchAgentIn(parts[1], u.Watches, "use."+u.Name)
			}
		}
		return Watch{}, fmt.Errorf("%q: no use named %q", target, useName)
	}
	return Watch{}, fmt.Errorf("%q is not <watch>, <use>.<watch> or self.<watch>", target)
}

// launchAgentIn refuses anything but a launchagent watch: the label and the
// plist path both come from it, so there is nothing to act on without one.
func launchAgentIn(name string, watches []Watch, where string) (Watch, error) {
	var agents []string
	for _, w := range watches {
		if w.Kind == WatchLaunchAgent {
			agents = append(agents, w.Name)
		}
		if w.Name != name {
			continue
		}
		if w.Kind != WatchLaunchAgent {
			return Watch{}, fmt.Errorf("%q is a %s watch; agent: acts on a launchagent watch, which is where the label and the plist come from", name, w.Kind)
		}
		return w, nil
	}
	if len(agents) == 0 {
		return Watch{}, fmt.Errorf("no watch named %q, and %s declares no launchagent watch", name, where)
	}
	return Watch{}, fmt.Errorf("no watch named %q; the launchagent watches are %s", name, strings.Join(agents, ", "))
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
