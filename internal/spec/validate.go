package spec

import "fmt"

func (s *Spec) validate() error {
	if s.App.Name == "" {
		return fmt.Errorf("app.name: required")
	}
	if s.App.ID == "" {
		return fmt.Errorf("app.id: required (the bundle identifier, e.g. dev.example.menubar)")
	}
	if s.App.Icon == "" {
		return fmt.Errorf("app.icon: required (an SF Symbol name)")
	}
	if s.App.Interval <= 0 {
		return fmt.Errorf("app.interval: required and must be positive, got %v", s.App.Interval)
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
	for i, r := range s.Status {
		if r.When == "" && i != len(s.Status)-1 {
			return fmt.Errorf("status[%d]: a rule with no when: always matches, so it must be last; %d rule(s) after it can never apply", i, len(s.Status)-1-i)
		}
	}
	return validateItems(s.Menu, "menu")
}

func validateItems(items []Item, path string) error {
	for i, it := range items {
		p := fmt.Sprintf("%s[%d]", path, i)
		if len(it.Menu) > 0 && it.Action.Kind != ActionNone {
			return fmt.Errorf("%s: has a submenu and a %s action; opening a submenu supersedes the action, so it would never run", p, it.Action.Kind)
		}
		if err := validateItems(it.Menu, p+".menu"); err != nil {
			return err
		}
	}
	return nil
}

func (w Watch) validate() error {
	if w.Kind == WatchExists && w.JSON {
		return fmt.Errorf("watch.%s: json has no meaning on an exists watch, which only binds .ok", w.Name)
	}
	if w.Shape != nil && !w.JSON {
		return fmt.Errorf("watch.%s: shape describes decoded JSON, so it needs json: true", w.Name)
	}
	return nil
}
