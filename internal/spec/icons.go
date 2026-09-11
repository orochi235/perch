package spec

import "fmt"

// Icons is every icon the spec names, keyed by the path to report an error
// against. The app's own icon is always present; a status rule contributes one
// only where it overrides.
func (s *Spec) Icons() map[string]Icon {
	out := map[string]Icon{"app.icon": s.App.Icon}
	for i, r := range s.Status {
		if !r.Icon.IsZero() {
			out[fmt.Sprintf("status[%d].icon", i)] = r.Icon
		}
	}
	return out
}
