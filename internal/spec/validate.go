package spec

import "fmt"

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
		if err := it.Action.validate(p); err != nil {
			return err
		}
		if err := validateItems(it.Menu, p+".menu"); err != nil {
			return err
		}
	}
	return nil
}

func (a Action) validate(path string) error {
	switch a.Kind {
	case ActionRun:
		if len(a.Run) == 0 {
			return fmt.Errorf("%s.run: empty; run takes the argv of a command", path)
		}
	case ActionPost:
		if a.PostURL == "" {
			return fmt.Errorf("%s.post: needs a url", path)
		}
	}
	return nil
}

func (w Watch) validate() error {
	path := "watch." + w.Name
	if err := checkName("watch", path, w.Name); err != nil {
		return err
	}
	if w.Name == "it" {
		return fmt.Errorf("%s: each: binds its element to `it`, so a watch cannot take that name", path)
	}
	if w.Kind == WatchRun && len(w.Run) == 0 {
		return fmt.Errorf("%s.run: empty; run takes the argv of a command", path)
	}
	if w.Kind == WatchExists && w.JSON {
		return fmt.Errorf("%s: json has no meaning on an exists watch, which only binds .ok", path)
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
