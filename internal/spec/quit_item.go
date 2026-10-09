package spec

// addQuit appends a Quit item when nothing in the menu quits. A status item
// has no Dock icon and no ⌘Q, so without one it cannot be closed from the UI.
func addQuit(menu []Item) []Item {
	if quits(menu) {
		return menu
	}
	const path = "menu (added by perch)"
	return append(menu,
		Item{Separator: true, path: path},
		Item{Text: "Quit", Action: Action{Kind: ActionQuit}, path: path})
}

func quits(items []Item) bool {
	for _, it := range items {
		if it.Action.Kind == ActionQuit || quits(it.Menu) {
			return true
		}
	}
	return false
}
