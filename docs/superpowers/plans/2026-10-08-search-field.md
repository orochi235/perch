# Search Field Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status: planned, not built.**

**Goal:** A `field:` menu item: a search field in the dropdown whose Enter fires the item's action with the typed text bound as `query`.

**Architecture:** The spec parser gains a `field:` key and its refusals. `celswift` gains a `query` binding and a URL-aware template lowering. `render.go` emits `.field(placeholder, { q in <action> })`, a closure because the text is not known when the menu is built. The Swift runtime gains `MenuNode.field` and a `FieldItem` (an `NSMenuItem` whose view hosts an `NSSearchField`) in a new `runtime/Field.swift`. The preview driver and docs site learn to draw it.

**Tech Stack:** Go (generator, `go test`), Swift/AppKit (emitted runtime, compiled by `swiftc` inside tests), cel-go AST.

**Spec:** `docs/superpowers/specs/2026-10-08-search-field-design.md`

## Global Constraints

- `query` is bound only inside a `field:` item's action; anywhere else it is a build error.
- A `field:` item takes exactly one of `run`, `open`, `post`, `swift`; `agent`, `quit`, `window`, `text`, `icon`, `menu` are refused at build time.
- `when` and `each` are allowed on a `field:` item; inside `each`, `it` and `query` are both bound.
- In `open:`, a hole that is exactly `{{query}}` is percent-encoded; other holes are untouched; `run:` and `post:` get the text raw.
- A `swift:` action on a field calls `Type.method(query)`; the method is `static func m(_ query: String)`.
- Enter with text closes the menu, runs the action, re-polls. Enter on an empty or whitespace-only field does nothing. Text is not kept between openings.
- `query` becomes a name perch binds itself, refused as a watch, state, or use name, like `it` and `self`. (No `menubar.yaml` under `~/src` uses it, checked 2026-10-08.)
- Runtime Swift files over a few hundred lines are not grown: the field's AppKit code goes in its own file, `runtime/Field.swift`.
- Tests run per package (`go test ./internal/spec/`, etc.), never the full suite locally. The full suite runs on the fleet with `onto test` after merge.

## Review Focus

- **Reserved URL characters in the typed text** (`a&b c+d#e`, `é`): `open:` must deliver it as one parameter. `CharacterSet.urlQueryAllowed` leaves `&`, `+`, and `=` alone, so the encoder uses an explicit unreserved set. Pinned in Task 1.
- **Whitespace-only text**: Enter does nothing. Pinned in Task 1.
- **A field inside a template (`use:`)**: `celswift`'s `inTemplate()` only allows `self` and `it`, so `query` would be misreported as "a template sees only self". Pinned in Task 3.
- **A field inside `each:` and inside a submenu**: `it` and `query` both resolve, and the closure captures the loop variable. Pinned in Task 4 (typecheck) and Task 1 (submenu focus, in the gated probe).
- **Shell-looking text in `run:`** (`; rm -rf ~`): stays one argv element and is never a shell. Pinned in Task 4 (emitted argv is `["onto", "find", "\(query1)"]`).

---

### Task 1: Runtime: `FieldItem`, `MenuNode.field`, URL encoder

The focus behavior is the risk named in the spec, so this task comes first. If the gated focus probe in Step 7 cannot be made to pass, stop and report back before Task 2. Don't work around it.

**Files:**
- Create: `internal/backend/swiftappkit/runtime/Field.swift`
- Modify: `internal/backend/swiftappkit/runtime/Runtime.swift` (`MenuNode`, `Draw.menu`, `Act`)
- Modify: `internal/backend/swiftappkit/runtime/Preview.swift` (`encoded(_ node:)`)
- Modify: `internal/backend/swiftappkit/emit.go` (embed and always emit `Field.swift`)
- Modify: `internal/backend/swiftappkit/golden_test.go:38` (skip `Field.swift` as fixed)
- Modify: `Package.swift` (exclude `Field.swift` from PerchKit)
- Test: `internal/backend/swiftappkit/field_test.go`

**Interfaces:**
- Produces (Swift): `MenuNode.field(String, (String) -> MenuAction)`; `final class FieldItem: NSMenuItem` with `init(placeholder: String, submit: @escaping (String) -> Void)`, `let field: NSSearchField`, `func enter()`; `Act.urlQuery(_ s: String) -> String`.
- Produces (Go): `fieldSwift []byte`, included in `runtimeFiles()`.

- [ ] **Step 1: Write the failing headless probe test**

```go
package swiftappkit

import (
	"os/exec"
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
		"drawn=true\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/backend/swiftappkit/ -run TestFieldItemHandsOnTextAndEncodesIt -v`
Expected: FAIL compiling the probe: `cannot find 'FieldItem' in scope`.

- [ ] **Step 3: Write `runtime/Field.swift`**

```swift
import AppKit

/// A menu item that takes typed text. Enter hands the text on and closes the
/// menu; an empty or blank field does nothing.
final class FieldItem: NSMenuItem {
    let field = NSSearchField()
    private let submit: (String) -> Void

    init(placeholder: String, submit: @escaping (String) -> Void) {
        self.submit = submit
        super.init(title: placeholder, action: nil, keyEquivalent: "")
        field.placeholderString = placeholder
        field.sendsWholeSearchString = true
        field.target = self
        field.action = #selector(enter)
        let host = FocusView(frame: NSRect(x: 0, y: 0, width: 240, height: 30))
        field.frame = host.bounds.insetBy(dx: 14, dy: 4)
        field.autoresizingMask = [.width]
        host.addSubview(field)
        host.focus = field
        view = host
    }

    @available(*, unavailable)
    required init(coder: NSCoder) { fatalError("not supported") }

    @objc func enter() {
        let text = field.stringValue
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }
        menu?.cancelTracking()
        submit(text)
    }
}

private final class FocusView: NSView {
    weak var focus: NSView?

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        guard let window, let focus else { return }
        // The menu's window takes first responder only once tracking has begun.
        DispatchQueue.main.async { window.makeFirstResponder(focus) }
    }
}
```

- [ ] **Step 4: Extend `Runtime.swift`**

Add the case to `MenuNode` (after `.submenu`):

```swift
    case field(String, (String) -> MenuAction)
```

Add to `Draw.menu`'s switch, after the `.submenu` case:

```swift
            case .field(let placeholder, let make):
                menu.addItem(FieldItem(placeholder: placeholder) { text in
                    Act.perform(make(text), then: repoll)
                })
```

Add to `enum Act`, after `open`:

```swift
    /// Typed text as one query component. urlQueryAllowed leaves & + = alone,
    /// which would split the text into several parameters.
    static func urlQuery(_ s: String) -> String {
        let unreserved = CharacterSet(charactersIn:
            "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
        return s.addingPercentEncoding(withAllowedCharacters: unreserved) ?? ""
    }
```

- [ ] **Step 5: Extend `Preview.swift` and `emit.go`**

In `Preview.swift`'s `encoded(_ node:)` switch, after `.submenu`. A preview has no typed text, so the action is shown as it would run on the literal word `query`:

```swift
    case .field(let placeholder, let make):
        out = ["field": placeholder, "action": encoded(make("query"))]
        icon = nil
```

In `emit.go`, beside the other embeds:

```go
//go:embed runtime/Field.swift
var fieldSwift []byte
```

and make `runtimeFiles()` return it too:

```go
	return []backend.File{
		{Name: "Runtime.swift", Body: runtimeSwift},
		{Name: "Icon.swift", Body: iconSwift},
		{Name: "Field.swift", Body: fieldSwift},
	}
```

In `golden_test.go:38`, add `|| f.Name == "Field.swift"` to the fixed-file skip. In `Package.swift`, add `"Field.swift"` to PerchKit's `exclude:` list.

- [ ] **Step 6: Run the probe test and the existing runtime tests**

Run: `go test ./internal/backend/swiftappkit/ -run 'TestFieldItem|TestDraw|TestRuntimeBehavior|TestSeparators|TestGolden' -v`
Expected: PASS. `swift build` is the PerchKit check: run `swift build` at the repo root. Expected: builds.

- [ ] **Step 7: Write the gated focus probe**

This is the spike. It opens a real status-item menu, so it is gated like the window tests. Add to `field_test.go`:

```go
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
let nodes: [MenuNode] = CommandLine.arguments.contains("submenu")
    ? [.submenu("More", [.field("Search", { q in .swift({ print("submitted=\(q)"); exit(0) }) })])]
    : [.field("Search", { q in .swift({ print("submitted=\(q)"); exit(0) }) })]
Draw.menu(nodes, into: menu, repoll: {})
status.menu = menu

let check = Timer(timeInterval: 0.5, repeats: false) { _ in
    let target = CommandLine.arguments.contains("submenu") ? menu.items[0].submenu!.items[0] : menu.items[0]
    let field = (target as! FieldItem).field
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
		t.Errorf("top level: got\n%s\nwant\n%s", out, want)
	}
}
```

For the submenu case, the timer has to open the submenu first, and AppKit gives no supported call for that. Run the top-level case as written. Then check the submenu case by hand in Step 9, not in this test.

- [ ] **Step 8: Run the gated test**

Run: `PERCH_WINDOW_TESTS=1 go test ./internal/backend/swiftappkit/ -run TestFieldTakesFocusWhenTheMenuOpens -v`
Expected: PASS. If `focused=false`, try these in order, rerunning after each:
1. Call `makeFirstResponder` from `NSMenuDelegate.menuWillOpen` via a delegate `Draw.menu` sets.
2. Schedule it with `RunLoop.main.perform(inModes: [.eventTracking])` instead of `DispatchQueue.main.async`.
3. Override `acceptsFirstResponder` on `FocusView`.

If none of them works, stop and report.

- [ ] **Step 9: Check focus by hand**

Write a scratch `menubar.yaml` in the session scratchpad with a top-level field and a field in a submenu, each `open:`ing `https://github.com/search?q={{query}}`. This needs Tasks 2–4, so do this step at the end of Task 4. Run `perch run -C <scratch>`, open the menu, and type without clicking. Report what happened for both fields.

- [ ] **Step 10: Commit**

```bash
git add internal/backend/swiftappkit/runtime/Field.swift internal/backend/swiftappkit/runtime/Runtime.swift internal/backend/swiftappkit/runtime/Preview.swift internal/backend/swiftappkit/emit.go internal/backend/swiftappkit/golden_test.go internal/backend/swiftappkit/field_test.go Package.swift
git commit -m "add a search field menu item to the Swift runtime"
```

---

### Task 2: Spec: parse `field:` and refuse what it cannot take

**Files:**
- Modify: `internal/spec/menu.go` (`Item`, `itemFields`, `parseItem`)
- Modify: `internal/spec/name.go:71-77` (`checkBound`)
- Modify: `internal/spec/quit.go:69` (a quit button decodes the same `itemFields`, so it must refuse `field:`)
- Test: `internal/spec/menu_test.go`, `internal/spec/state_test.go`, `internal/spec/quit_test.go`

**Interfaces:**
- Produces: `spec.Item.Field string`, which is the placeholder, with `{{ }}` holes allowed. Non-empty means the item is a field.

- [ ] **Step 1: Write the failing tests** (append to `menu_test.go`)

```go
func TestParseField(t *testing.T) {
	items := menuOf(t, `  - field: Search GitHub
    open: "https://github.com/search?q={{query}}"`)
	if items[0].Field != "Search GitHub" || items[0].Action.Kind != ActionOpen {
		t.Errorf("got %+v", items[0])
	}
}

func TestParseFieldRefusals(t *testing.T) {
	for name, tc := range map[string]struct{ item, want string }{
		"no action": {`{field: Find}`, "needs an action"},
		"quit":      {`{field: Find, quit: true}`, "cannot take a quit"},
		"agent":     {`{field: Find, agent: w.stop}`, "cannot take an agent"},
		"window":    {`{field: Find, window: open}`, "cannot take a window"},
		"text":      {`{field: Find, text: Go, open: x}`, "takes no text"},
		"icon":      {`{field: Find, icon: star, open: x}`, "takes no icon"},
		"submenu":   {"field: Find\n    menu: [{text: a}]", "takes no submenu"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte("app: {name: a, id: b, icon: c, interval: 1s}\nmenu:\n  - " + tc.item + "\n"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to say %q", err, tc.want)
			}
		})
	}
}
```

Add `"strings"` to `menu_test.go`'s imports. Append to `state_test.go`:

```go
func TestQueryIsBoundByPerch(t *testing.T) {
	_, err := Parse([]byte("app: {name: a, id: b, icon: c, interval: 1s}\nwatch: {query: {exists: /tmp}}\nmenu: [{text: a}]\n"))
	if err == nil || !strings.Contains(err.Error(), "bound by perch itself") {
		t.Errorf("err = %v", err)
	}
}
```

Append to `quit_test.go` (match its existing document shape for `app.quit`):

```go
func TestQuitButtonRefusesAField(t *testing.T) {
	_, err := Parse([]byte(`
app:
  name: a
  id: b
  icon: c
  interval: 1s
  quit:
    - confirm: Stop?
      buttons: [{field: Why, run: [x]}]
menu: [{text: a}]
`))
	if err == nil || !strings.Contains(err.Error(), "field") {
		t.Errorf("err = %v", err)
	}
}
```

The submenu case has no action. Make sure its error comes from the submenu refusal, not "needs an action": the submenu check runs first.

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/spec/ -run 'TestParseField|TestQueryIsBoundByPerch' -v`
Expected: FAIL with `unknown key "field"`.

- [ ] **Step 3: Implement**

In `menu.go`, add `Field string` to `Item` (after `Text`), and `Field string \`yaml:"field"\`` to `itemFields`. In `parseItem`, after the existing submenu-and-action check, call the new check and set the field:

```go
	if f.Field != "" {
		if err := checkField(f, act, sub, icon, path); err != nil {
			return Item{}, err
		}
	}
	return Item{Text: f.Text, Field: f.Field, Icon: icon, When: f.When, Each: f.Each, Menu: sub, Action: act, path: path}, nil
```

```go
// checkField refuses what a field: item cannot use: everything but an action
// that can take the typed text.
func checkField(f itemFields, act Action, sub []Item, icon Icon, path string) error {
	switch {
	case len(sub) > 0:
		return fmt.Errorf("%s: a field: item takes no submenu; Enter cannot open one", path)
	case f.Text != "":
		return fmt.Errorf("%s: a field: item takes no text:; its placeholder is its label", path)
	case !icon.IsZero():
		return fmt.Errorf("%s: a field: item takes no icon:; the search field draws its own", path)
	}
	switch act.Kind {
	case ActionRun, ActionOpen, ActionPost, ActionSwift:
		return nil
	case ActionNone:
		return fmt.Errorf("%s: a field: item needs an action to hand the text to: run, open, post or swift", path)
	}
	return fmt.Errorf("%s: a field: item cannot take %s action; it does nothing with typed text. Use run, open, post or swift", path, withArticle(act.Kind.String()))
}
```

In `quit.go`'s `parseQuitButton`, extend the existing refusal:

```go
	if f.When != "" || f.Each != "" || f.Menu.Kind != 0 || f.Field != "" {
		return QuitButton{}, fmt.Errorf("%s: a button takes a label and at most one action; when, each, menu and field have no meaning on one", path)
	}
```

In `name.go`'s `checkBound`, add the third binder:

```go
	binder := map[string]string{
		"it":    "each: binds its element to it",
		"self":  "a template binds its own use to self",
		"query": "a field: item binds its typed text to query",
	}[name]
```

- [ ] **Step 4: Run the package's tests**

Run: `go test ./internal/spec/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/spec/
git commit -m "parse field: menu items and reserve query as a bound name"
```

---

### Task 3: celswift: bind `query`, encode it in URLs

**Files:**
- Modify: `internal/celswift/env.go` (`WithQuery`, `Query`, `inTemplate`)
- Modify: `internal/celswift/template.go` (`LowerURL`)
- Modify: `internal/celswift/lower.go:117-127` (unknown-name message for `query`)
- Test: `internal/celswift/template_test.go`, `internal/celswift/refusal_test.go`

**Interfaces:**
- Produces: `func (e *Env) WithQuery(swiftName string) *Env`; `func (e *Env) Query() (string, bool)`, which returns the Swift name; `func (e *Env) LowerURL(src string) (string, error)`.

- [ ] **Step 1: Write the failing tests** (append to `template_test.go`)

```go
func TestLowerURLEncodesOnlyABareQueryHole(t *testing.T) {
	e := shaped(t).WithQuery("query1")
	got, err := e.LowerURL("https://x/?q={{query}}&n={{fleet.data.label}}")
	if err != nil {
		t.Fatalf("LowerURL: %v", err)
	}
	want := `"https://x/?q=\(Act.urlQuery(query1))&n=\(fleet.data.label)"`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestLowerURLWithoutAQueryIsLowerTemplate(t *testing.T) {
	a, _ := shaped(t).LowerURL("https://x/{{fleet.data.label}}")
	b, _ := shaped(t).LowerTemplate("https://x/{{fleet.data.label}}")
	if a != b {
		t.Errorf("LowerURL %q, LowerTemplate %q", a, b)
	}
}

func TestQueryIsAStringInRunArgs(t *testing.T) {
	got, err := runWatch(t).WithQuery("query1").LowerTemplate("{{query}}")
	if err != nil || got != `"\(query1)"` {
		t.Errorf("got %q, %v", got, err)
	}
}
```

Before writing them, check that `shaped`'s shape has a string field named `label`. `TestLowerTemplateKeepsStringHolesUncast` uses `fleet.data.label`, so it does. Append to `refusal_test.go`:

```go
func TestQueryOutsideAFieldSaysWhereItIsBound(t *testing.T) {
	_, err := runWatch(t).LowerTemplate("{{query}}")
	if err == nil || !strings.Contains(err.Error(), "only in a field: item's action") {
		t.Errorf("err = %v", err)
	}
}

func TestQueryResolvesInsideATemplate(t *testing.T) {
	u := spec.Use{Name: "d"}
	if _, err := ForUse(u, "d").WithQuery("query1").LowerTemplate("{{query}}"); err != nil {
		t.Errorf("a field inside a template cannot see query: %v", err)
	}
}
```

Make sure `refusal_test.go` imports `strings` and the `spec` package.

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/celswift/ -run 'LowerURL|Query' -v`
Expected: FAIL: `WithQuery` undefined.

- [ ] **Step 3: Implement**

`env.go`:

```go
// WithQuery returns a copy of e with query bound to a field's typed text, for
// lowering that field's action.
func (e *Env) WithQuery(swiftName string) *Env {
	out := &Env{vars: append([]binding(nil), e.vars...)}
	out.vars = append(out.vars, binding{name: "query", typ: &spec.Type{Kind: spec.TypeString}, swift: swiftName, local: true})
	return out
}

// Query is how the typed text is spelled in Swift, when e lowers a field's action.
func (e *Env) Query() (string, bool) {
	b, ok := e.lookup("query")
	return b.swift, ok
}
```

In `inTemplate`, change `case "it":` to `case "it", "query":`.

`template.go`: rename the body of `LowerTemplate` to `lowerTemplate(src string, url bool)` and add:

```go
func (e *Env) LowerTemplate(src string) (string, error) { return e.lowerTemplate(src, false) }

// LowerURL is LowerTemplate for an open: target: a hole that is exactly query
// is percent-encoded, so the typed text arrives as one query parameter.
func (e *Env) LowerURL(src string) (string, error) { return e.lowerTemplate(src, true) }
```

Inside `lowerTemplate`, after `lowered` is computed:

```go
		if url && expr == "query" {
			if _, ok := e.Query(); ok {
				lowered = "Act.urlQuery(" + lowered + ")"
			}
		}
```

`lower.go`, in the `IdentKind` not-found branch, beside the `self` case:

```go
			if name == "query" {
				return value{}, fmt.Errorf("%q: query is bound only in a field: item's action, where it is the typed text", src)
			}
```

- [ ] **Step 4: Run the package's tests**

Run: `go test ./internal/celswift/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/celswift/
git commit -m "bind query in a field's action and percent-encode it in open: URLs"
```

---

### Task 4: Render: emit `.field(…)`

**Files:**
- Modify: `internal/backend/swiftappkit/render.go` (`body`, `action`, new `field`)
- Modify: `internal/backend/swiftappkit/typecheck_test.go` (new doc in the map)
- Modify: `internal/backend/swiftappkit/golden_test.go` (new `field` golden)
- Create: `internal/backend/swiftappkit/testdata/field/` (via `-update`)
- Test: `internal/backend/swiftappkit/field_test.go`, `internal/backend/swiftappkit/swift_action_test.go`

**Interfaces:**
- Consumes: `spec.Item.Field`; `Env.WithQuery`, `Env.Query`, `Env.LowerURL`; Swift `MenuNode.field`, `Act.urlQuery`.

- [ ] **Step 1: Write the failing tests**

Add to `field_test.go`:

```go
const fieldDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
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
```

Add `"strings"` to the imports. The suffixes `query1`/`query2` depend on `g.name`'s counter. If the `each:` loop variable takes a number first, adjust the expected strings to what the emitter produces. Check that the counter is the only thing that differs.

In `swift_action_test.go`, add a second doc and compile test that mirror the existing pair:

```go
const swiftFieldDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {field: Find, swift: Dash.find}
`
```

The hand-written source for it is:

```swift
import AppKit

enum Dash {
    static func find(_ query: String) {
        NSLog("%@", query)
    }
}
```

It asserts that `Render.swift` contains `.swift({ Dash.find(query1) })`, and that `swiftc` compiles the emitted files beside `Dash.swift`. Factor the existing test's compile body into a helper `compileWithHand(t, doc, hand string)` that both tests call, rather than copying it.

Add `"field": fieldDoc` to `TestGolden`'s map and to `TestEmittedSwiftTypechecks`'s map.

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/backend/swiftappkit/ -run 'TestFieldLowers|TestSwiftAction' -v`
Expected: FAIL. With no field branch, the field items lower as text items with an empty title.

- [ ] **Step 3: Implement**

In `render.go`'s `body`, right after the separator case:

```go
	if it.Field != "" {
		return g.field(it, into, e, path)
	}
```

```go
// field lowers the action inside a closure: the text it hands on is typed after
// the menu is built, so it cannot be lowered as data like other actions.
func (g *menuGen) field(it spec.Item, into string, e *celswift.Env, path string) error {
	placeholder, err := e.LowerTemplate(it.Field)
	if err != nil {
		return fmt.Errorf("%s.field: %w", path, err)
	}
	q := g.name("query")
	action, err := g.action(it.Action, e.WithQuery(q), path, it.Scope)
	if err != nil {
		return err
	}
	g.b.line("%s.append(.field(%s, { %s in %s }))", into, placeholder, q, action)
	return nil
}
```

In `action`, the `ActionOpen` case switches `e.LowerTemplate(a.Open)` to `e.LowerURL(a.Open)`. That keeps one pathway, since `LowerURL` only differs when `query` is bound. The `ActionSwift` case becomes:

```go
	case spec.ActionSwift:
		// Emitted verbatim; perch cannot check the target exists, swiftc does.
		if q, ok := e.Query(); ok {
			return ".swift({ " + a.Swift + "(" + q + ") })", nil
		}
		return ".swift(" + a.Swift + ")", nil
```

- [ ] **Step 4: Write the golden and run the package**

Run: `go test ./internal/backend/swiftappkit/ -run TestGolden -update`, then read `testdata/field/Render.swift` to confirm it says what Step 1 expects.
Run: `go test ./internal/backend/swiftappkit/ -run 'TestField|TestSwiftAction|TestGolden|TestEmittedSwiftTypechecks|TestTemplatedSwiftTypechecks'`
Expected: PASS. Typecheck compiles the `each:` field's closure that captures `it`. That covers the `each:` item in Review Focus.

- [ ] **Step 5: Do Task 1, Step 9 (focus by hand)**, now that YAML reaches the runtime.

- [ ] **Step 6: Commit**

```bash
git add internal/backend/swiftappkit/
git commit -m "emit field: items as closures over the typed text"
```

---

### Task 5: Preview and docs site draw the field

**Files:**
- Modify: `internal/preview/preview.go` (`Node.Field`)
- Modify: `internal/site/menu.go` (`nodeHTML`)
- Modify: `internal/site/assets/site.css` (near line 317, the `.row` rules)
- Test: `internal/preview/preview_test.go`, `internal/site/site_test.go`

**Interfaces:**
- Consumes: Preview JSON `{"field": <placeholder>, "action": {...}}` from Task 1.
- Produces: `preview.Node.Field string`.

- [ ] **Step 1: Write the failing tests**

In `preview_test.go`, model it on `TestRenderCoversTheOtherWatchKindsAndActions`:

```go
const fieldPreviewDoc = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
menu:
  - field: Search GitHub
    open: "https://github.com/search?q={{query}}"
`

func TestRenderShowsAFieldAndWhatItWouldOpen(t *testing.T) {
	s := parse(t, fieldPreviewDoc)
	frames := render(t, s, []State{state(t, s, "any", "")})
	n := frames[0].Menu[0]
	if n.Field != "Search GitHub" || n.Action == nil || n.Action.Open != "https://github.com/search?q=query" {
		t.Errorf("got %+v", n)
	}
}
```

If `state(t, s, name, "")` refuses an empty document for a file with no watches, use whatever the existing no-watch tests pass.

In `site_test.go`:

```go
func TestMenuDrawsAFieldAsASearchRow(t *testing.T) {
	got := nodeHTML("", preview.Node{Field: "Search", Action: &preview.Action{Open: "x"}})
	if !strings.Contains(got, `class="row field"`) || !strings.Contains(got, "Search") {
		t.Errorf("got %s", got)
	}
}
```

- [ ] **Step 2: Run them and confirm they fail**

Run: `go test ./internal/preview/ ./internal/site/ -run 'Field' -v`
Expected: FAIL. `Node` has no field `Field`.

- [ ] **Step 3: Implement**

`preview.go`, in `Node`: `Field string \`json:"field,omitempty"\``.

`menu.go`, in `nodeHTML`, after the separator case:

```go
	if n.Field != "" {
		tip := ""
		if n.Action != nil {
			tip = ` title="` + html.EscapeString(actionText(*n.Action)) + `"`
		}
		return `<div class="row field"` + tip + `><span class="input">` + html.EscapeString(n.Field) + `</span></div>`
	}
```

`site.css`, after `.row.label`:

```css
.row.field .input { display: block; padding: 2px 8px; border: 1px solid var(--ink-faint); border-radius: 6px; color: var(--ink-faint); }
```

- [ ] **Step 4: Run both packages**

Run: `go test ./internal/preview/ ./internal/site/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/preview/ internal/site/
git commit -m "draw field: items in previews and on the docs site"
```

---

### Task 6: Editor schema, reference docs, retire the spec

**Files:**
- Modify: `internal/schema/schema.go` (menu item properties near line 202; reserved-name enums at lines 69 and 98)
- Modify: `docs/schema.md` (`## menu`, `### Actions`, the reserved-names wording)
- Modify: `docs/superpowers/HANDOFF.md`
- Delete: `docs/superpowers/specs/2026-10-08-search-field-design.md`, this plan
- Test: `internal/schema/drift_test.go` (existing), `internal/e2e/docs_test.go` (existing)

- [ ] **Step 1: Run the drift test and confirm it fails**

Run: `go test ./internal/schema/`
Expected: FAIL in `TestSchemaDeclaresExactlyTheKeysTheParserAccepts`, naming `field`. If it passes, the drift test does not reach item keys. Add a case for `field` there before going on.

- [ ] **Step 2: Update the schema**

Beside `"text"` in the menu item properties (line 202):

```json
"field": {"type": "string", "description": "A search field; this is its placeholder. Enter fires the item's run, open, post or swift with the typed text bound as query. {{ }} holes are CEL."},
```

Add `"query"` after `"self"` in both reserved-name `enum` lists (lines 69 and 98). Rerun: `go test ./internal/schema/`. Expected: PASS.

- [ ] **Step 3: Write the reference**

In `docs/schema.md`'s `## menu` key table, add after `text`:

```markdown
| `field` | A string: the placeholder of a search field drawn in the menu. `{{ }}` holes interpolate expressions. See [`field`](#field). |
```

Add a `### field` section after `### each`. Move the spec's substance here: the example, the allowed and refused keys table, the escaping rule, `swift:` taking `(_ query: String)`, and what Enter, an empty field, and Esc do. Write it in the reference's own register: what a person writes and sees, no design rationale. Wherever the doc lists the names perch binds (`it`, `self`), add `query`.

- [ ] **Step 4: Run the docs checks**

Run: `go test ./internal/e2e/ -run Docs` and `go test ./internal/site/`
Expected: PASS. The docs tests compile the examples in `schema.md`, so the new example has to build.

- [ ] **Step 5: Retire the scaffolding**

Delete the spec and this plan. In `HANDOFF.md`, remove the "Search field is designed, not built" entry. Add under **Landed**: "**Search field** (unreleased): a `field:` menu item; see [`field`](../schema.md#field)."

- [ ] **Step 6: Commit**

```bash
git add -A internal/schema docs
git commit -m "document field: menu items and retire the search field design"
```

After merging to main, start `onto test` in the background (the `onto-test` skill) and read its exit code when it finishes. Leave the release and version bump to the maintainer.
