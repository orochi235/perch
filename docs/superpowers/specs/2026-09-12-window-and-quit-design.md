# A window, a Swift action, and quit policy

**Status: designed, not built.** Nothing below is in the schema yet. This is
the design to build from; the reader is whoever builds or changes it.

Three additions, driven by the last repo not on perch. `window:` puts a WebKit
window behind a menu item. `swift:` lets a menu item call hand-written Swift.
`app.quit:` decides what quitting asks first. Two bugs in the installer are
fixed on the way, because a window is what exposes them.

## Why

`~/src/pw/reviewplex` is a local dashboard for GitHub pull requests with a menu
bar helper in front of it: 904 lines across seven hand-written Swift files. Most
of it perch already says better. Its `ServerMonitor` is two watches, one `http`
on `/api/health` and one JSON on `/api/summary`. Its `ServiceControl` — a
`launchctl` wrapper with a 200ms memo, added only because a menu validating
three items probed three times — is one `launchagent:` watch and three `agent:`
actions. Icon, dimming and inbox badge are `state:` and `status:`.

Three things have nowhere to go, and all three are about a window.

## The window is more declarable than the design said

[The design](2026-09-05-perch-design.md) puts embedded web views out of scope,
and names this window as the example: *a window whose reload behavior depends on
whether its last navigation failed.* That reason does not survive reading it.
`loadFailed` is set from `WKNavigationDelegate` callbacks and read in `show()`.
It never leaves the window. No expression, no watch and nothing an author writes
would ever name it — it is entanglement with WebKit's navigation state, not with
state the schema cannot name.

Everything an author would choose is small:

```yaml
window:
  url: http://localhost:4747/
  title: reviewplex
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}   # optional; omit for no zoom controls
```

`size:` is width then height. One window per app, matching one status item per
app, which is why nothing names it: the top-level `window:` block declares the
window, and `window:` on a menu item acts on it — `open`, `close` or `reload`.

Everything else is fixed, because it is right for every consumer and each one is
a trap:

| Behavior | Why it is not a knob |
|---|---|
| Close hides, never destroys | A destroyed WebView loses scroll position and every expanded or filtered thing; `isReleasedWhenClosed = false` is what keeps reopening cheap |
| Reopen reloads only if the last load failed | Held open across a server restart the page is dead HTML; reloading unconditionally throws away the state the previous row buys |
| A load is always fresh, never `reload()` | `reload()` does nothing when the first navigation failed and left no back-forward entry |
| Present deminiaturizes first | `orderOut` does not clear the miniaturized flag, so a window hidden while minimized comes back with no tile to restore it from |
| Dock presence follows visibility | Raised before activating, or the activation lands on an app with no tile; lowered after ordering out, or demoting while frontmost strands the menu bar |

Declaring `window:` also emits a main menu — App, File, Edit, View, Window. Not
optional. An `.accessory` app owns no menu bar until it is `.regular`, and
without the Edit menu ⌘C does not work inside the WebView at all: the standard
selectors reach `WKWebView` through the responder chain, and wiring that menu is
the whole of copy and paste. ⌘Q binds to closing the window; quitting moves to
⌥⌘Q. For a menu-bar-first app ⌘Q otherwise costs the status item and its
polling, which is not what the muscle memory is buying. Not ⇧⌘Q either — the
Apple menu reserves it for Log Out and swallows it before the app's item sees
the key.

The `menu:` block is mirrored as one more top-level menu, titled `app.name`. One
declaration drives the status menu and the menu bar, so the two cannot disagree
about what is currently possible — which is the property the hand-written
version documents itself as wanting and maintains by hand. The cost is that
read-only lines like `Server: running on :4747` appear as disabled items in a
real menu bar.

## swift:

The [`Sources/` seam](../../schema.md#hooking-the-app-from-sources) reserves
three `NSApplicationDelegate` lifecycle methods for hand-written extensions on
`Controller`. It is how brainhouse keeps notification delivery. It is also
launch-only: hand-written code can *start* at launch, and nothing can *invoke*
it from a menu.

A new action closes that:

```yaml
menu:
  - {text: Preferences…, swift: Dash.showPreferences}
```

Takes a dotted path and emits `Type.method()`. Dotted rather than bare so
hand-written code cannot collide with emitted symbols — `Controller`, `Results`,
`Draw`, `Watcher`, `renderFace`, `renderMenu`.

The method takes nothing and returns nothing. Handing it the poll results would
bind hand-written code to generated struct names that change whenever a watch is
added or renamed, which is the coupling the `Generated`/`Sources` line exists to
prevent.

perch cannot typecheck it; `swiftc` does, because the emitted call compiles in
the same module as `menubar/Sources/*.swift`. A missing type or a misspelled
method is a compile failure — so `perch build` alone will not catch it, but
`run` and `install` will. Like every other action it re-polls afterward. Unlike
them it raises no alert on failure: there is no exit status to inspect, so error
reporting belongs to the hook.

## app.quit:

reviewplex's confirmation hangs off `applicationShouldTerminate` rather than its
menu item, deliberately: the Dock tile's own Quit calls `terminate` directly and
skips anything wired to a menu. With a `window:` block there is always a tile.
So quit's confirmation is app policy, not item decoration — and the item stays
`{text: Quit, quit: true}` carrying one action.

An ordered list, first match wins, last rule bare — the idiom `status:` already
uses:

```yaml
app:
  name: reviewplex
  id: com.reviewplex.menubar
  icon: tray
  interval: 5s
  quit:
    - when: svc.loaded
      confirm: "Quit reviewplex?"
      detail: "Stops the menu bar item and its polling. The server is running as a launchd service."
      buttons:
        - {text: Quit Both, agent: svc.stop}
        - {text: Quit Helper Only}
    - confirm: "Quit reviewplex?"
      detail: "The menu bar item goes away and stops watching the server. The server itself keeps running."
```

A button may carry one action — the same ones a menu item takes, less `quit:`
and `window:`, neither of which means anything while the app is already
terminating — run before terminating. If it fails the quit is
canceled and the alert names the command — the user asked for both to go, and if
the server is still up they need to see that rather than lose the status item
and assume it worked. Cancel is implicit and always last; the first button is
the default; a rule with no `confirm:` quits without asking.

Every route in goes through it: the menu item, ⌥⌘Q, the Dock tile,
`NSApp.terminate` from anywhere.

Logout, restart and shutdown skip the prompt entirely. Generated policy, never
declared — a helper that cancels someone's logout is worse than one that exits
without asking, a modal raised during shutdown is a dialog nobody is there to
dismiss, and no author should have to learn that this is a category of bug. The
check reads the quit reason off the current Apple event: `kAELogOut`,
`kAEReallyLogOut`, `kAEShutDown`, `kAERestart`, `kAEQuitAll`. An absent event or
an absent reason means a quit aimed at this app, because `NSApp.terminate` from
our own menus and from the Dock tile carries no reason at all.

`applicationShouldTerminate` is now perch's, and stays off the reserved seam
list. A consumer implementing it would fight the generated confirm.

## Two installer bugs a window exposes

**`KeepAlive: true` makes Quit a no-op.** `internal/install/plist.go` writes an
unconditional `KeepAlive`, so quitting exits 0 and launchd restarts the app at
once. Every example in the docs ends with `{text: Quit, quit: true}`; none of
them work when installed. It becomes `KeepAlive: {SuccessfulExit: false}` —
restart on a crash, stay quit on a clean exit. Independent of everything else
here and worth landing first.

**There is no app icon.** `internal/install/bundle.go` copies `menubar/Icons/`
into `Resources` for the status item and writes no `.icns` and no
`CFBundleIconFile`. Invisible while `LSUIElement` keeps the app out of the Dock;
a `window:` app promotes to `.regular` and gets a tile with a blank generic
icon. Install grows the pipeline reviewplex has in bash: ten renditions from
`menubar/AppIcon.png` via `sips`, `iconutil -c icns`, and a `touch` of the
bundle afterward, because LaunchServices caches icons per bundle and a changed
icon otherwise does not appear.

The source is a fixed path rather than a schema key. The file being there is
already the whole of the decision, and a key would be a second way to say it.

`LSUIElement: true` stays hardcoded and correct. The app launches as
`.accessory` and promotes at runtime; the plist key decides where it starts, not
where it may go.

## What this does not do

**The seam keeps its justification, narrowed.** `seam_test.go` names "an
embedded web view" as a case the seam exists for. After this, it does not.
Notification delivery remains: banners fire on a transition rather than on a
condition holding, and the cursor that makes them fire once is state no `when:`
can name. The "Out of scope" section of the design doc needs rewriting to say
so.

**`swift:` does not reach the app delegate.** A hand-written type could be named
in YAML and have lifecycle callbacks routed through it. That is a partial eject:
perch would still own the app but no longer know what it does, and every later
feature would carry an "unless the hook overrode it" clause. The full eject is
already free. A call-out keeps generated code in charge.

**No second window, no window without a menu bar, no preferences UI.** One
status item per app was already the rule; one window is its match.

**Zoom is not persisted.** The frame is, through `setFrameAutosaveName` keyed on
`app.name`. Zoom resets to 1.0 on launch, as it does today.

## What changes

| File | Change |
|---|---|
| `internal/spec/spec.go` | `App.Quit` |
| `internal/spec/quit.go` | new: quit rules, buttons, one action per button |
| `internal/spec/window.go` | new: `Window`, `url`, `title`, `size`, `zoom` |
| `internal/spec/menu.go` | `ActionSwift`, `ActionWindow`, dotted-path parse |
| `internal/spec/validate.go` | one `window:` per doc; `window:` actions need the block; bare quit rule last |
| `internal/backend/swiftappkit/main.go` | `applicationShouldTerminate`, `applicationShouldHandleReopen` |
| `internal/backend/swiftappkit/window.go` | new: the window, its main menu |
| `runtime/Runtime.swift` | `WebWindow`, `MainMenu`, `Act.swift`, `Act.window`, the confirm alert |
| `internal/install/plist.go` | `KeepAlive: {SuccessfulExit: false}` |
| `internal/install/bundle.go` | `.icns`, `CFBundleIconFile`, the post-write `touch` |
| `internal/schema/schema.go` | `window`, `swift`, `app.quit` |
| `docs/schema.md` | the three blocks; the seam section loses web views |
| `docs/recipes/` | a window recipe |
| `2026-09-05-perch-design.md` | "Out of scope" rewritten |

## Testing

The usual shape — a refusal table in `internal/spec`, goldens carried through
`swiftc -typecheck`, recipe states rendered by compiling what perch emits — plus
four that need a real Aqua session, because nothing else can tell the
difference:

- **Quit stays quit.** Install a widget, activate its Quit item, assert the
  process does not come back. Fails today against `KeepAlive: true`, which is
  the point.
- **The Dock tile follows the window.** Open, assert `.regular`; close, assert
  `.accessory`.
- **A reopen after the server goes away reloads.** Load, stop the server, close,
  restart it, reopen, assert the page is live rather than WebKit's error page.
- **A hand-written `swift:` target compiles and runs**, beside the existing seam
  test, which keeps asserting the three lifecycle methods stay unimplemented.

`applicationShouldTerminate` under a real logout cannot be tested without
logging out. The quit-reason check gets a unit test against synthesized Apple
event descriptors instead, and the gap is named here.
