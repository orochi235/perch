package spec

import "testing"

const quitDoc = `
app:
  name: w
  id: dev.example.w
  icon: gear
  interval: 5s
  quit:
    - when: svc.loaded
      confirm: "Quit w?"
      detail: "Stops the menu bar item and its polling."
      buttons:
        - {text: Quit Both, agent: svc.stop}
        - {text: Quit Helper Only}
    - confirm: "Quit w?"
      detail: "The server keeps running."
watch:
  svc:
    launchagent: dev.example.svc
menu:
  - {text: Quit, quit: true}
`

func TestQuitRulesParse(t *testing.T) {
	s, err := Parse([]byte(quitDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.App.Quit) != 2 {
		t.Fatalf("got %d rules, want 2", len(s.App.Quit))
	}
	first := s.App.Quit[0]
	if first.When != "svc.loaded" || first.Confirm != "Quit w?" {
		t.Errorf("first rule = %+v", first)
	}
	if len(first.Buttons) != 2 {
		t.Fatalf("got %d buttons, want 2", len(first.Buttons))
	}
	if first.Buttons[0].Text != "Quit Both" || first.Buttons[0].Action.Kind != ActionAgent {
		t.Errorf("first button = %+v", first.Buttons[0])
	}
	if first.Buttons[1].Action.Kind != ActionNone {
		t.Errorf("second button carries an action: %+v", first.Buttons[1].Action)
	}
	if s.App.Quit[1].When != "" {
		t.Error("the last rule has a condition, so nothing is the fallback")
	}
}

func TestQuitRefusals(t *testing.T) {
	head := "app:\n  name: w\n  id: dev.example.w\n  icon: gear\n  interval: 5s\n  quit:\n"
	tail := "watch:\n  svc:\n    launchagent: dev.example.svc\nmenu:\n  - {text: Quit, quit: true}\n"
	cases := map[string]string{
		"bare rule not last":  "    - {confirm: A}\n    - {when: x, confirm: B}\n",
		"no confirm":          "    - {detail: nothing to ask}\n",
		"button with no text": "    - {confirm: A, buttons: [{agent: svc.stop}]}\n",
		"button quits":        "    - {confirm: A, buttons: [{text: X, quit: true}]}\n",
		"button opens window": "    - {confirm: A, buttons: [{text: X, window: open}]}\n",
		"button has a when":   "    - {confirm: A, buttons: [{text: X, when: svc.loaded}]}\n",
		"unknown key":         "    - {confirm: A, detial: x}\n",
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(head + rules + tail))
			if err == nil {
				t.Fatal("accepted; want a refusal")
			}
		})
	}
}

func TestQuitIsOptional(t *testing.T) {
	s, err := Parse([]byte("app: {name: w, id: dev.example.w, icon: gear, interval: 5s}\nmenu:\n  - {text: Quit, quit: true}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.App.Quit) != 0 {
		t.Errorf("got %d rules from a doc with no quit:", len(s.App.Quit))
	}
}
