package spec

// verbLabel is what an agent: item says when it has no text:.
var verbLabel = map[AgentVerb]string{
	AgentStart:   "Start",
	AgentStop:    "Stop",
	AgentRestart: "Restart",
}

// verbGuard is when a verb can work, written against the watch the item acts
// on. start is bootstrap, which fails on a label launchd already holds.
func verbGuard(target string, v AgentVerb) string {
	switch v {
	case AgentStart:
		return target + ".installed && !" + target + ".loaded"
	case AgentStop:
		return target + ".loaded"
	}
	return target + ".installed"
}

// applyVerbDefaults labels every agent: item that has no text:, so a menu
// never offers a verb that fails.
func applyVerbDefaults(items []Item) {
	for i := range items {
		it := &items[i]
		applyVerbDefaults(it.Menu)
		if it.Action.Kind != ActionAgent {
			continue
		}
		if it.Text == "" {
			it.Text = verbLabel[it.Action.Verb]
		}
		guard := verbGuard(it.Action.Agent, it.Action.Verb)
		if it.When == "" {
			it.When = guard
		} else {
			// A "\n" before the close paren keeps a trailing "//" comment in
			// an author's when: from swallowing the guard that follows it.
			it.When = "(" + it.When + "\n) && (" + guard + ")"
		}
	}
}
