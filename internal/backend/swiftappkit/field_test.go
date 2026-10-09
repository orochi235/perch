package swiftappkit

import (
	"os/exec"
	"strings"
	"testing"
)

// fieldProbe drives a FieldItem without opening a menu: what Enter hands on,
// and what the URL encoder makes of text a person might type.
const fieldProbe = `
import AppKit

var got: [String] = []
let item = FieldItem(placeholder: "Search") { got.append($0) }
print("placeholder=\(item.field.placeholderString ?? "")")
for text in ["", "   ", "a&b c"] {
    item.field.stringValue = text
    item.enter()
}
print("submitted=\(got)")
print("encoded=\(Act.urlQuery("a&b c+d#e=é/?"))")
print("json=\(Act.jsonEscaped("say \"hi\" \\ é\n\u{1}"))")

let menu = NSMenu()
Draw.menu([.field("Find", { q in .open("x:" + q) })], into: menu, repoll: {})
print("drawn=\(menu.items.first is FieldItem)")
`

func TestFieldItemHandsOnTextAndEncodesIt(t *testing.T) {
	bin := buildProbe(t, fieldProbe)
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	want := "placeholder=Search\n" +
		`submitted=["a&b c"]` + "\n" +
		"encoded=a%26b%20c%2Bd%23e%3D%C3%A9%2F%3F\n" +
		`json=say \"hi\" \\ é\n\u0001` + "\n" +
		"drawn=true\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

// fieldFocusProbe opens a real status item menu and checks the field took the
// keyboard: a field you must click first is not a search bar. It types no keys,
// which would need Accessibility; it asks AppKit who holds focus, then edits
// through the field editor that focus implies.
const fieldFocusProbe = `
import AppKit

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let status = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
status.button?.title = "probe"
let menu = NSMenu()
Draw.menu([.field("Search", { q in .swift({ print("submitted=\(q)"); exit(0) }) })], into: menu, repoll: {})
status.menu = menu

let check = Timer(timeInterval: 0.5, repeats: false) { _ in
    let field = (menu.items[0] as! FieldItem).field
    let editor = field.currentEditor()
    print("focused=\(editor != nil && field.window?.firstResponder === editor)")
    editor?.insertText("a&b c")
    field.sendAction(field.action, to: field.target)
}
RunLoop.main.add(check, forMode: .common)
let timeout = Timer(timeInterval: 10, repeats: false) { _ in print("timeout"); exit(1) }
RunLoop.main.add(timeout, forMode: .common)
DispatchQueue.main.async { status.button?.performClick(nil) }
app.run()
`

func TestFieldTakesFocusWhenTheMenuOpens(t *testing.T) {
	requireAqua(t)
	requireScreen(t)
	bin := buildProbe(t, fieldFocusProbe)
	out, _ := exec.Command(bin).CombinedOutput()
	if want := "focused=true\nsubmitted=a&b c\n"; string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

const fieldDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s, hotkey: cmd+shift+space}
watch:
  fleet:
    run: [onto, top, --json]
    json: true
    shape: {jobs: [{id: string, node: string}]}
menu:
  - field: Search GitHub
    open: "https://github.com/search?q={{query}}"
  - field: Find job
    run: [onto, find, "{{query}}"]
  - each: fleet.data.jobs
    field: "Grep {{it.node}}"
    post: {url: "http://localhost/{{it.id}}", body: {q: "{{query}}"}}
  - {text: Quit, quit: true}
`

func TestFieldLowersToAClosureOverTheText(t *testing.T) {
	r := emit(t, fieldDoc)["Render.swift"]
	for _, want := range []string{
		`.field("Search GitHub", { query1 in .open("https://github.com/search?q=\(Act.urlQuery(query1))") })`,
		`.field("Find job", { query2 in .run(["onto", "find", "\(query2)"]) })`,
		`.field("Grep \(`,
	} {
		if !strings.Contains(r, want) {
			t.Errorf("Render.swift lacks %s\n%s", want, r)
		}
	}
}
