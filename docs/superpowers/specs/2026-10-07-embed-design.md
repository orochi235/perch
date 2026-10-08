# Embedding

**Status: designed, not built.**

This is the design for whoever builds embed mode and whoever changes it next.
It answers: how an app that already has its own process and status item uses
perch's YAML, conditions, chip, and tint without perch running anything.

## Why

brent and colm (`~/src/brent`, `~/src/colm`) are Swift menu bar apps whose
menus are hand-written `NSMenuItem` code. They want what perch gives a widget,
but inside their own process; a second app polling them would be the wrong
shape, since they already hold the state the menu shows.

So perch gets two modes. A standalone file gets all of perch: polling, launchd,
the `.app`. An embed file gets a compiler: YAML in, Swift out, and perch is out
of the way at runtime. The YAML stays the source; nobody edits the generated
Swift.

## The YAML

A file with `input:` is an embed file. `input:` is a [shape](../../schema.md#shape):
each field is a top-level name in expressions, as a watch name is, and together
they are the generated `Model`. brent's menu:

```yaml
app: {name: brent, icon: figure.run.square.stack, tint: {color: "#116C80", style: chip}}
input:
  host: string
  routed: bool
  mouse: string
  keyboard: string
  peers: string
  secure: bool
  grants: [{title: string, pane: string}]
  warnings: [string]
  showGaze: bool
  gazeDemo: bool
  switcherFollowsGaze: bool
menu:
  - text: "brent on {{host}}"
  - {text: "mouse → {{mouse}}", when: routed}
  - {text: "keyboard → {{keyboard}}", when: routed}
  - {text: "peers: {{peers}}", when: routed}
  - {text: "secure input is on: keys stay here", when: secure}
  - {each: grants, text: "⚠ {{it.title}}", open: "x-apple.systempreferences:com.apple.preference.security?{{it.pane}}"}
  - {each: warnings, text: "⚠ {{it}}"}
  - separator
  - {text: Show gaze screen, checked: showGaze, call: toggleOutline}
  - {text: Gaze demo mode, checked: gazeDemo, call: toggleDemo}
  - {text: Cmd-Tab opens where you look, checked: switcherFollowsGaze, call: toggleSwitcher}
  - {text: "Bring everything home (⌃⌥⌘H)", call: goHome}
  - {text: Quit brent, quit: true, key: q}
```

| | Embed file | Standalone file |
|---|---|---|
| `input:` | Required | Refused |
| `watch:`, `use:`, `window:` | Refused: the generated Swift has no polling or window code | As before |
| `app:` | `name`, `icon`, `tint` only | As before |
| `state:`, `status:`, `menu:` | As before | As before |
| `call: <name>` (new) | Runs the host's closure of that name | Refused; `swift:` does this |
| `swift:` | Allowed; compiles only if the files share the host's module | As before |
| `checked:` (new) | A CEL condition; a checkmark while it holds | Allowed |
| `key:` (new) | A one-character key equivalent | Allowed |

The `tint` user default reads the host's own defaults. Shapes have no optional
fields and CEL here has no `join`, so a host passes `routed: bool` and a joined
string; both are left until a second host needs them.

## The Swift

perch writes these into the output directory:

| File | Holds |
|---|---|
| `Menu.swift` | The drawing half of today's `Runtime.swift`: `Face`, `MenuNode`, `Draw`, and the `open`, `post`, `run`, and `quit` actions. Standalone apps get it too. |
| `Icon.swift` | Icons and tint, as before. |
| `Shapes.swift` | `Model` and a struct per object in its shape. |
| `Render.swift` | As before, reading `Model` instead of `Results`. |
| `Menubar.swift` | The class the host calls. |

The polling half of `Runtime.swift` (watchers, launchd, quit prompts) becomes
`Poll.swift`, emitted only for standalone apps.

```swift
let menubar = Menubar(model: Model(host: Host.current().localizedName ?? "?"),
                      calls: .init(toggleOutline:  { [weak self] in self?.flip(.showGaze) },
                                   toggleDemo:     { [weak self] in self?.flip(.gazeDemo) },
                                   toggleSwitcher: { [weak self] in self?.flip(.switcherFollowsGaze) },
                                   goHome:         { [weak self] in self?.goHome() }))
menubar.update(model)   // whenever the host's state changes
```

- `Menubar` is `@MainActor`. It creates the status item when made and removes it
  when released.
- `update(_:)` redraws the face at once; the menu is rebuilt from the latest
  `Model` each time it opens.
- `Model`, its nested structs, `Menubar`, `Calls`, `init`, and `update` are
  public; everything else is internal. `Model` has a default for every field.
- `Calls` holds one non-optional closure per `call:` name, so a missing handler
  is a build error in the host rather than a menu item that does nothing.
- An action is not followed by a poll, as there is none; the menu changes when
  the host next calls `update`.

## How a host builds it

perch is a SwiftPM build tool plugin. The host adds perch as a package and puts
the plugin on a target holding `menubar.yaml`; each `swift build` regenerates
the Swift when the YAML changed, and nothing of the plugin is linked.

```swift
.package(url: "https://github.com/orochi235/perch", from: "2.3.0"),
.target(name: "BrentMenu", plugins: [.plugin(name: "Perch", package: "perch")]),
.executableTarget(name: "Brent", dependencies: ["BrentCore", "BrentNet", "BrentMenu"]),
```

- `Sources/BrentMenu/` holds `menubar.yaml` and one placeholder `.swift` file:
  a target whose only sources come from a plugin fails with "unable to resolve
  module dependency" (measured with Xcode 27's SwiftPM, 2026-10-07).
- Generated sources compile into the plugin's target, so `BrentMenu` is its own
  module and the runtime's type names never meet the host's.
- The plugin runs `perch build -o <plugin work directory>` on the target's
  `menubar.yaml`, declaring the YAML as input and the generated files as
  outputs. Editing the YAML regenerates on the next build (measured, same day).
- `perch build` on an embed file writes the same files by hand, for a host that
  would rather check them in.

`Package.swift` sits at the root of the perch repo, so perch's tags are the
package's versions. A plugin cannot build Go, so it runs a prebuilt perch from a
binary target: a `.artifactbundle.zip` holding arm64 and x86_64 binaries,
attached to each GitHub release and named in `Package.swift` by URL and
checksum. A release builds the bundle, writes its checksum into `Package.swift`,
commits, then tags; the binary comes from the same Go source as the tag. This
sits beside the Homebrew tap bump.

## What it does not do

- Optional shape fields and CEL `join`.
- Xcode-project hosts: Xcode runs package plugins, but this design is not tested
  against one.
- Converting brent: that goes back to the brent session once perch releases this.

## Testing

- A golden test of brent's YAML in `testdata/embed/`.
- `swiftc -typecheck` on the embed output alone, and with a host file that
  builds a `Model`, fills `Calls`, and calls `update`, so the public surface is
  proven from outside the module.
- Parser tests for each refusal in the table, and `checked:` and `key:` in both
  modes.
- The editor schema learns `input:`, `call:`, `checked:`, and `key:`; the drift
  tests hold it to the parser.
- A plugin test: a scratch package using the plugin through a local path builds
  and runs.
- Docs: a schema section on embedding, with brent's example and the
  `Package.swift` lines; the site's previews draw checkmarks.
