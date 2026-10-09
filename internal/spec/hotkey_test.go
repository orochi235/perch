package spec

import (
	"strings"
	"testing"
)

func parseAppHotkey(src string) (Hotkey, error) {
	doc := "app: {name: a, id: b, icon: c, interval: 1s, hotkey: " + `"` + src + `"` + "}\nmenu: [{text: a}]\n"
	s, err := Parse([]byte(doc))
	if err != nil {
		return Hotkey{}, err
	}
	return s.App.Hotkey, nil
}

func TestHotkeyParses(t *testing.T) {
	for src, want := range map[string]Hotkey{
		"cmd+shift+space": {Cmd: true, Shift: true, Key: "space"},
		"ctrl+opt+k":      {Ctrl: true, Opt: true, Key: "k"},
		"cmd+/":           {Cmd: true, Key: "/"},
		"f13":             {Key: "f13"},
		"shift+f5":        {Shift: true, Key: "f5"},
	} {
		got, err := parseAppHotkey(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got != want {
			t.Errorf("%s: got %+v, want %+v", src, got, want)
		}
	}
}

func TestHotkeyStringRoundTrips(t *testing.T) {
	h, err := parseAppHotkey("cmd+shift+ctrl+opt+a")
	if err != nil {
		t.Fatal(err)
	}
	if got := h.String(); got != "ctrl+opt+shift+cmd+a" {
		t.Errorf("got %s", got)
	}
}

func TestNoHotkeyIsUnset(t *testing.T) {
	s, err := Parse([]byte("app: {name: a, id: b, icon: c, interval: 1s}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.App.Hotkey.IsSet() {
		t.Errorf("got %+v", s.App.Hotkey)
	}
}

func TestHotkeyRefuses(t *testing.T) {
	for src, want := range map[string]string{
		"cmd+":             "empty part",
		"cmd+shift":        "ends in a modifier",
		"alt+space":        `write "alt" as opt`,
		"command+k":        `write "command" as cmd`,
		"hyper+k":          "is not a modifier",
		"Cmd+K":            `in lowercase, as cmd+k`,
		"cmd+cmd+k":        "written twice",
		"cmd+enter":        `write "enter" as return`,
		"cmd+numlock":      "is not a key perch knows",
		"k":                "from every app",
		"shift+k":          "from every app",
		"space":            "from every app",
		"cmd+shift+space+": "empty part",
	} {
		_, err := parseAppHotkey(src)
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "app.hotkey") {
			t.Errorf("%s: got %v, want an app.hotkey error containing %q", src, err, want)
		}
	}
}
