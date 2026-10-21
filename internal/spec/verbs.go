package spec

// verbLabel is what an agent: item says when it has no text:.
var verbLabel = map[AgentVerb]string{
	AgentStart:   "Start",
	AgentStop:    "Stop",
	AgentRestart: "Restart",
	AgentEnable:  "Start at Login",
	AgentDisable: "Don't Start at Login",
}

// verbGuard is when a verb can work, written against the watch the item acts
// on. start is bootstrap, which fails on a label launchd already holds.
func verbGuard(target string, v AgentVerb) string {
	switch v {
	case AgentStart:
		return target + ".installed && !" + target + ".loaded"
	case AgentStop:
		return target + ".loaded"
	case AgentEnable:
		return target + ".installed && !" + target + ".enabled"
	case AgentDisable:
		return target + ".installed && " + target + ".enabled"
	}
	return target + ".installed"
}

// applyVerbDefaults labels every agent: and toggle: item that has no text: and
// guards it, so a menu never offers a verb that fails.
func applyVerbDefaults(items []Item) {
	for i := range items {
		it := &items[i]
		applyVerbDefaults(it.Menu)
		switch {
		case it.Action.Kind == ActionAgent:
			if it.Text == "" {
				it.Text = verbLabel[it.Action.Verb]
			}
			it.Guard = verbGuard(it.Action.Agent, it.Action.Verb)
		case !it.Toggle.IsZero():
			if it.Text == "" {
				it.Text = verbLabel[AgentEnable]
				if it.Toggle.Negated {
					it.Text = verbLabel[AgentDisable]
				}
			}
			it.Guard = it.Toggle.Target + ".installed"
		}
	}
}
