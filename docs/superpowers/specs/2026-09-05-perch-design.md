# perch — a menu bar app from a YAML file

perch is a code generator. It reads `menubar.yaml`, emits a macOS status-bar
app in Swift, builds it into a `.app`, and loads it as a LaunchAgent.

This document is the design. It is written for whoever picks the repo up next,
including its author six months on. It answers: what the schema can say, what
it deliberately cannot, and where the generated code stops and hand-written
code begins.

**Status: built.** All six commands work, and the generated app has been installed, reloaded over a running copy, and removed on real hardware. No repo consumes it yet.

## Why

Two menu bar apps exist — `brainhouse/menubar/main.swift` and
`the-other-app/menubar/Sources/` — and `onto` wants a third. The two are the
same program with the nouns swapped: matching `ServerState` and `ServiceState`
enums, and a `menuNeedsUpdate` that agrees comment for comment.

The expensive part was never the Swift. It is the wrapper: compile, fake an app
bundle so `UNUserNotificationCenter` will run at all, set `LSUIElement`, write a
launchd plist, and work around `bootout` returning before launchd has let go.
That is 80 lines of bash in one repo and 97 in the other.

The two are not the same 80 lines, and that is the actual argument. the other app
carries two fixes brainhouse never got: it builds into a temp directory so a
failed `swiftc` cannot damage a working install, and it removes the bundle
before rebuilding so a renamed or dropped resource cannot linger. A fix made in
a copy stays in that copy.

## Two languages

perch is written in Go and emits Swift. They are not alternatives: Go is what
you run, Swift is what comes out.

Swift is not a preference. `NSStatusItem`, `NSMenu`, SF Symbols and
`NSMenuDelegate`'s rebuild-on-open are AppKit, reachable first-class only from
Swift and Objective-C. Go's alternative is `getlantern/systray`, a cgo wrapper
over a lowest-common-denominator menu — icons as raw PNG bytes, no SF Symbols,
and no hook to rebuild the menu as it opens, which is the mechanism both
existing apps rely on to never offer an action that cannot work. Being cgo, it
does not even buy a pure-Go binary.

Go is right for the generator because the backend seam is real: a Linux target
emits something that is not Swift, and a Swift program is the wrong thing to
put in charge of that.

## Shape

Three stages:

    menubar.yaml  ──parse──▶  Spec  ──emit──▶  Swift  ──install──▶  running .app

The validated `Spec` *is* the intermediate representation. A second,
target-neutral IR is not built until a second backend exists to justify one.
The seam that matters today is the backend interface, not another data
structure behind it.

    type Backend interface {
        Name() string
        Emit(*spec.Spec) ([]File, error)
    }

Installing is not on the interface. Bundling and launchd are driven by the app's
identity rather than by the spec, so they live in `internal/install` and the
command orchestrates them; a second backend brings its own installer the same
way.

`swift-appkit` is the only backend. Others get a name and nothing else.

## Codegen, not a runtime interpreter

The alternative was one installed app reading YAML at launch. Codegen wins on
one property: **ejection is free.** Both existing apps grew things a schema
would choke on — an embedded WebKit window with a `Cmd-W`/`Cmd-R` event
monitor, a notification cursor seeded from the first poll so a restart does not
replay banners. A runtime would have to either express all of that or invent an
escape hatch. Codegen's escape hatch is "stop running the generator."

Generated Swift is committed, so a repo builds without perch installed.

## Schema

```yaml
app:
  name: onto
  id: dev.onto.menubar
  icon: rectangle.3.group        # SF Symbol
  interval: 5s

watch:                           # polled concurrently every interval
  fleet:
    run: [onto, top, --once, --json]
    json: true
    shape:                       # optional; without it, .data is untyped
      nodes: [{name: string, up: bool}]
      jobs:  [{id: string, node: string, cmd: string}]

status:                          # first matching rule wins
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - when: "fleet.data.jobs.size() == 0"
    dim: true
  - badge: "fleet.data.jobs.size()"

menu:
  - text: "{{fleet.data.nodes.size()}} nodes · {{fleet.data.jobs.size()}} running"
  - separator
  - each: fleet.data.jobs
    text: "{{it.node}} — {{it.cmd}}"
    menu:
      - {text: Logs,  run: [onto, logs, "{{it.id}}"]}
      - {text: Prune, run: [onto, prune, "{{it.id}}"]}
      - {text: Kill,  run: [onto, kill, "{{it.id}}"]}
  - separator
  - {text: Quit, quit: true}
```

Two shapes the schema refuses, because both would otherwise fail silently: a
`status:` rule with no `when:` must be last, since nothing after it can ever
match; and an item cannot carry both an action and a `menu:`, since opening a
submenu supersedes the action.

### Watches

Three kinds, which is what it takes to cover all three apps:

| Kind | Binds |
|---|---|
| `run: [argv]` | `.ok` (exit 0), `.code`, `.out`, `.err`, and `.data` when `json: true` |
| `http: <url>` | `.ok` (2xx), `.status`, `.out`, `.data` |
| `exists: <path>` | `.ok` |

A watch that fails sets `.ok` false and leaves `.data` null. It never crashes
the app, and the menu still opens.

`brainhouse` needs no more than this: `http:` for the server, `run:` on
`launchctl print` for whether launchd holds the job, `exists:` on the plist to
tell "installed but unloaded" from "not installed". Those three states are the
whole of what its menu branches on today.

### Actions

Four verbs. `run` takes argv and never a shell. `open` takes a URL or a path.
`post` takes a URL and a JSON body. `quit` ends the app.

Every action re-polls immediately on completion — `brainhouse` does this by
hand and it is what makes Start feel like it did something. A non-zero exit
raises an alert naming the command, as `runLaunchctlOrAlert` does today.

`launchctl` control needs no dedicated support. Start, Stop and Restart are
three `run:` items with `when:` guards.

### Menus

Menus rebuild on open from the last poll results, which is what both existing
apps already do and the reason their actions never offer something impossible.
`each:` repeats an item over a list, binding the element to `it`. `when:` on
any item decides whether it appears.

## Expressions are CEL

Every `when:`, every `badge:`, and every `{{ }}` hole is a
[CEL](https://github.com/google/cel-spec) expression, parsed by `cel-go` at
build time and lowered to a plain Swift expression. **Nothing evaluates CEL at
runtime** — no interpreter ships in the app, and a malformed expression is a
build error rather than a widget that silently shows nothing.

This is the one decision taken from `a schema engine elsewhere`, which reached for CEL
for the same reason: an authored condition language, invented ad hoc, grows
into a bad programming language. Reusing that engine itself was rejected — it
models fields in a document, emits no code, and is Python.

### The supported subset

Field selection and indexing, literals, `== != < <= > >=`, `&& || !`, `in`,
the ternary `? :`, and the functions `size()`, `has()`, `string()`,
`startsWith()`, `contains()`. Anything else is refused at build time with the
offending expression quoted. perch does not claim to implement CEL; it claims
to reject what it has not implemented.

Four edges the lowering settled, none of which widen that list. A comparison
between an `int` and a `double` is refused rather than quietly widened. Both
arms of a `? :` must have the same type. `in` against a declared list requires
the element type to match. And `has()` on a field a shape declares is a
build-time constant, because the shape has already said the field is there.
There is no `null` literal.

Lowering targets a small `JSONValue` shim emitted into `Generated/` alongside
the app.

### Watch shapes

An undeclared watch binds `.data` as `dyn`, and then CEL checks grammar, arity
and operators but not field names: `fleet.data.jobs` and `fleet.data.jbos`
compile the same, and the second produces a widget that silently shows nothing.

A watch may therefore declare the shape of its output, which becomes CEL's type
environment and turns a misspelled field into a build error. The vocabulary is
`string`, `int`, `double`, `bool`, `any`, objects, and lists written `[T]`. No
enums, no unions, no cardinality, no presentation — a shape describes what a
command prints, and nothing else.

Declaring one is optional per watch. `brainhouse` counts one integer out of an
HTTP body and wants none of this; `onto` walks two lists of records and does.

Two things follow from having a shape, and are the reason it earns its place
over a comment:

- The emitted Swift decodes into a generated struct rather than subscripting
  `JSONValue`, so the shim shrinks to the untyped watches and Swift's own
  compiler checks the lowering.
- `perch shape --from '<command>'` runs the command once and writes the
  declaration from what came back. Optional typing normally goes unused because
  authoring it is a chore; here the chore is one command.

## Files, in a consuming repo

    menubar.yaml           authored
    menubar/Generated/     emitted, committed, header says regenerating overwrites
    menubar/Sources/       hand-written, perch never reads or writes it

Ejecting is moving a file across that line and deleting the YAML that made it.
The directory boundary is the whole of the drift story: nothing outside
`Generated/` is ever overwritten, and nothing inside it is ever authored.

A `# yaml-language-server: $schema=` header points at a JSON Schema perch
emits, so authoring gets completion and inline errors in an editor.

## Commands

    perch build         emit Swift into menubar/Generated/
    perch run           build, compile, run in the foreground (the dev loop)
    perch install       build, compile, bundle, write the plist, bootstrap
    perch uninstall     bootout and remove
    perch shape         run a watch once and write its shape declaration
    perch schema        write the JSON Schema for menubar.yaml

`install` owns the bundle identity, `LSUIElement`, and the
bootout-wait-bootstrap loop. That loop exists because `bootout` returns before
launchd has let go, and bootstrapping into the gap fails with `Input/output
error`. It is written once here instead of once per repo.

## Out of scope

Notification delivery and embedded web views are hand-written Swift, not
schema. `onto` needs neither, and both existing uses of them are entangled with
state the schema has no way to name — a cursor seeded from the first poll, a
window whose reload behavior depends on whether its last navigation failed.

One status item per app. No preferences UI. No auto-update.

## Testing

Golden tests hold YAML in and Swift out. Each golden's output is additionally
run through `swiftc -typecheck`, because a golden test alone stays green while
emitting Swift that does not compile — the failure mode where a check answers
the question next to the one being asked.

CEL lowering is tested separately, expression in and Swift expression out, with
its own cases for every construct outside the subset asserting a diagnostic
rather than silence.

## First consumer: onto

`onto` is a controller with three enrolled nodes and no local agent, so its
widget answers "is onto doing anything right now" rather than "is a service
up". It needs one thing from `onto` itself: a `--json` output carrying nodes
and running jobs. `onto top` already performs exactly that fan-out on a timer,
so this is a formatter and not new machinery.

Every route but `GET /v1/info` is signature-gated, so the widget shells out to
`onto` and the ed25519 signing stays in Go where it already works.
