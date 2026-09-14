package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// QuitRule decides what quitting asks first. The first rule whose When holds
// wins; a rule with no When always matches, so it must be last.
//
// It is app policy rather than an item's, because the Dock tile's own Quit
// calls terminate directly and skips anything wired to a menu item.
type QuitRule struct {
	When    string
	Confirm string
	Detail  string
	// Buttons are offered in order; the first is the default and Cancel is
	// implicit and last. Empty offers one button named Quit.
	Buttons []QuitButton
}

// QuitButton is one answer. Its action runs before the app terminates, and a
// failure cancels the quit.
type QuitButton struct {
	Text   string
	Action Action
}

type rawQuitRule struct {
	When    string      `yaml:"when"`
	Confirm string      `yaml:"confirm"`
	Detail  string      `yaml:"detail"`
	Buttons []yaml.Node `yaml:"buttons"`
}

func parseQuit(n *yaml.Node) ([]QuitRule, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("app.quit: want a list of rules, first match wins")
	}
	out := make([]QuitRule, 0, len(n.Content))
	for i, c := range n.Content {
		path := fmt.Sprintf("app.quit[%d]", i)
		var raw rawQuitRule
		if err := decodeStrict(c, &raw, path); err != nil {
			return nil, err
		}
		rule := QuitRule{When: raw.When, Confirm: raw.Confirm, Detail: raw.Detail}
		for j := range raw.Buttons {
			b, err := parseQuitButton(&raw.Buttons[j], fmt.Sprintf("%s.buttons[%d]", path, j))
			if err != nil {
				return nil, err
			}
			rule.Buttons = append(rule.Buttons, b)
		}
		out = append(out, rule)
	}
	return out, nil
}

func parseQuitButton(n *yaml.Node, path string) (QuitButton, error) {
	var f itemFields
	if err := decodeStrict(n, &f, path); err != nil {
		return QuitButton{}, err
	}
	if f.When != "" || f.Each != "" || f.Menu.Kind != 0 {
		return QuitButton{}, fmt.Errorf("%s: a button takes a label and at most one action; when, each and menu have no meaning on one", path)
	}
	act, err := actionFrom(f, path)
	if err != nil {
		return QuitButton{}, err
	}
	switch act.Kind {
	case ActionQuit:
		return QuitButton{}, fmt.Errorf("%s: every button quits, so quit: on one says nothing", path)
	case ActionWindow:
		return QuitButton{}, fmt.Errorf("%s: the app is already terminating, so there is no window to %s", path, act.Window)
	}
	return QuitButton{Text: f.Text, Action: act}, nil
}

// validateQuit checks the ordering and the parts an expression cannot supply.
func (s *Spec) validateQuit() error {
	for i, r := range s.App.Quit {
		path := fmt.Sprintf("app.quit[%d]", i)
		if r.Confirm == "" {
			return fmt.Errorf("%s.confirm: required; a rule with nothing to ask is a rule that quits, which is what leaving the rule out already does", path)
		}
		if r.When == "" && i != len(s.App.Quit)-1 {
			return fmt.Errorf("%s: a rule with no when: always matches, so it must be last; %d rule(s) after it can never apply", path, len(s.App.Quit)-1-i)
		}
		for j, b := range r.Buttons {
			bp := fmt.Sprintf("%s.buttons[%d]", path, j)
			if b.Text == "" {
				return fmt.Errorf("%s.text: required; a button with no label cannot be told from Cancel", bp)
			}
			if err := b.Action.validate(bp, s.Watches, s.Window); err != nil {
				return err
			}
		}
	}
	return nil
}
