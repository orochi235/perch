package spec

import "fmt"

// Icons is every icon the spec names, keyed by the path to report an error
// against. The app's own icon is always present; a status rule or menu item
// contributes one only where it sets one, and a templated one not at all, since
// its name is not known until the app polls.
func (s *Spec) Icons() map[string]Icon {
	out := map[string]Icon{"app.icon": s.App.Icon}
	for i, r := range s.Status {
		if !r.Icon.IsZero() {
			out[fmt.Sprintf("status[%d].icon", i)] = r.Icon
		}
	}
	menuIcons(s.Menu, out)
	return out
}

func menuIcons(items []Item, out map[string]Icon) {
	for _, it := range items {
		if !it.Icon.IsZero() && !it.Icon.Templated() {
			out[it.Path()+".icon"] = it.Icon
		}
		menuIcons(it.Menu, out)
	}
}
