# `menubar.yaml`

The reference for the file perch reads. It is for someone writing one; for why
the schema stops where it does, see [the design](superpowers/specs/2026-09-05-perch-design.md).

A document has seven top-level keys — `app`, `watch`, `state`, `use`, `status`,
`window` and `menu`. Only `app` is required, though an app with no `menu` offers nothing but
its icon.

```yaml
# yaml-language-server: $schema=./menubar.schema.json
app:
  name: onto
  id: dev.onto.menubar
  icon: rectangle.3.group
  interval: 5s

watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true

status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - badge: "fleet.data.jobs.size()"

menu:
  - text: "{{fleet.data.jobs.size()}} running"
  - separator
  - {text: Quit, quit: true}
```

`perch schema -o menubar.schema.json` writes the JSON Schema that header points
at, which gets you completion and inline errors in an editor.

Unknown keys are rejected everywhere, so a typo is a build error rather than a
menu item that silently never appears.

## `app`

The first four keys are required.

| Key | Takes |
|---|---|
| `name` | A string, one path component: no `/`, no leading dot, not `.` or `..`. It names the `.app` under `~/Applications` and the executable inside it. |
| `id` | A string of letters, digits, dots, dashes and underscores, starting with a letter or digit — `dev.example.menubar`. launchd takes it as a label and `install` as a plist filename. |
| `icon` | An [SF Symbol](https://developer.apple.com/sf-symbols/) name, or `{asset: <name>}` where `<name>` is letters, digits, dashes and underscores. Shown when no `status:` rule overrides it. See [Icon assets](#icon-assets). |
| `interval` | A Go duration: a number and a unit, one or more times. The units are `ns`, `us`, `ms`, `s`, `m`, `h` — `5s`, `1m30s`. Must be positive. |
| `sign` | Optional. A string naming a code signing identity in your keychain. Omitted, the `.app` is signed ad-hoc — see [Signing](guide/install.md#signing-and-why-a-rebuild-can-lose-a-permission). |

### `quit`

What quitting asks first. An ordered list, first match wins, the last rule bare
— the idiom `status:` uses.

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
      detail: "The menu bar item goes away. The server itself keeps running."

watch:
  svc:
    launchagent: com.reviewplex

menu:
  - {text: Quit, quit: true}
```

| Key | Takes |
|---|---|
| `when` | A string: the condition. Omit it to match always, which means it must be last. |
| `confirm` | A string: the question. Required. |
| `detail` | A string: the smaller text under it. |
| `buttons` | A list of `{text: …}` mappings, each with at most one action — the same ones a menu item takes, less `quit:` and `window:`. |

Buttons are offered in order, the first is the default, and Cancel is implicit
and always last. A button's action runs before the app terminates; if it fails,
the alert names the command and the quit is canceled. With no `buttons:` the
prompt offers one button named Quit.

This is app policy rather than something on the Quit item, because the Dock
tile's own Quit calls `terminate` directly and would skip anything wired to a
menu. Every route in goes through it: the menu item, ⌥⌘Q, the Dock tile.

Logging out, restarting and shutting down skip the prompt and quit. A helper
that cancels someone's logout is worse than one that exits without asking.

Leave the block out and the app quits without asking.

## `watch`

A mapping of names to polled sources. They run concurrently, and they all
re-poll on the [`interval`](#app) the `app:` block sets. Their results are what
every expression in the document reads.

A name is letters, digits and underscores, starting with a letter or
underscore. `it` is taken — `each:` binds its element to that — and so is
`self`, which a template binds to its own use. So are CEL's own keywords, and
`init`, `Type` and `Protocol`, which the emitted Swift could not reach; those
three are refused as state, use and [shape](#shape) field names too.

Each watch is exactly one of four kinds. The kind is the key it carries:

| Key | Takes |
|---|---|
| `run` | A list of at least one string: argv. Never a shell, so no pipes, no globbing, no `~`. |
| `http` | A string: the URL to GET. |
| `exists` | A string: a path to test for. A leading `~` is expanded. |
| `launchagent` | A string: a launchd label, written like a bundle identifier — `dev.example.worker`. |

Three more keys qualify a kind rather than choosing one:

| Key | Takes | On |
|---|---|---|
| `json` | `true` or `false`. Decodes the output and binds `.data`. | `run`, `http` |
| `shape` | What that output looks like — see [shape](#shape). Needs `json: true`. | `run`, `http` |
| `plist` | A string: where the LaunchAgent file is. Defaults to `~/Library/LaunchAgents/<label>.plist`. | `launchagent` |

`run:` is a list rather than one string because a `{{ }}` hole would otherwise
decide how many arguments it becomes: splitting `onto kill {{it.id}}` on spaces
turns an id of `j1 extra` into two arguments. As a list, `"{{it.id}}"` is one
element whatever it holds.

What a watch binds depends on its kind:

| Kind | Binds |
|---|---|
| `run` | `.ok` (exit status 0), `.code` (int), `.out`, `.err` |
| `http` | `.ok` (a 2xx), `.status` (int), `.out` |
| `exists` | `.ok` |
| `launchagent` | `.installed`, `.loaded`, `.running` (bools), `.pid` (int), `.label`, `.plist`, `.domain`, `.target` |

An `exists` watch binds only `.ok`, so `json:` on one is an error rather than a
no-op.

What each `launchagent` field means, and why there is no `.ok`, is in
[LaunchAgents](#launchagents).

A watch that fails sets `.ok` false and leaves `.data` null. It never takes the
app down, and the menu still opens.

### `shape`

Without a declared shape, `.data` is untyped: `fleet.data.jobs` and
`fleet.data.jbos` both compile, and the second produces a widget that silently
shows nothing.

A shape says what the command prints, which becomes the type environment the
expressions are checked against — so the misspelling is a build error instead.

```
shape:
  nodes: [{name: string, up: bool}]
  jobs:  [{id: string, node: string, cmd: string}]
```

The vocabulary is `string`, `int`, `double`, `bool`, `any`, objects, and lists
written `[T]` with exactly one element. No enums, no unions, no optionality, no
presentation. A shape needs `json: true`, since it describes decoded JSON.

Shapes are optional per watch, and writing one is a chore, so perch writes it
for you:

```
perch shape --from 'onto top --once --json'
perch shape --sample healthy.json --sample failing.json
```

Both flags repeat, and several samples merge into one shape.

An inferred shape is a starting point, not an answer: it describes only the
samples it saw, and a healthy sample omits every field that appears only when
something is wrong — disproportionately the ones a widget branches on.

## `state`

An ordered list naming the conditions the widget can be in. The first whose
condition holds wins, and the last takes no condition at all — so exactly one
state holds at every poll.

Each entry is a mapping of exactly one key: the state's name, and the condition
that reaches it as a string. The last entry's value is nothing at all, which is
what makes it the fallback.

Each name is then a boolean any expression can use — every `when:`, every
`badge:`, every `{{ }}` hole.

```yaml
app: {name: worker, id: dev.example.worker.menubar, icon: gearshape, interval: 10s}

watch:
  worker:
    launchagent: dev.example.worker

state:
  - uninstalled: "!worker.installed"
  - stopped: "!worker.loaded"
  - running:

status:
  - {when: uninstalled, icon: exclamationmark.triangle}
  - {when: stopped,     dim: true}

menu:
  - {text: Not installed, when: uninstalled}
  - {text: Not loaded,    when: stopped}
  - {text: Running,       when: running}
  - {text: Quit, quit: true}
```

A state means "the widget is in this state", not "this condition holds" — the
two differ, because a state also excludes every state declared before it.
`stopped` above is `!uninstalled && !worker.loaded`, so the ordering is what
says an agent that is not installed is not also stopped.

The block is optional, and a name follows the rule a watch name follows:
letters, digits and underscores, starting with a letter or underscore, and not
`it`, `self`, `init`, `Type` or `Protocol`. States, watches and uses are named
the same way in an expression, so a state may not take a watch's or a use's
name.

A condition names watches, and the states of a [use](#use), and nothing else.
Naming another state from this list is refused rather than resolved: a state
declared earlier is already excluded by the ordering, so naming one could only
ever be a constant, and a guard that looks like a guard while contributing
nothing is the failure this schema exists to prevent. A use's states are a list
of their own, so naming one is not a constant.

## `use`

A mapping of names to templates. A template is a block of watches, states,
status rules and menu items written once; perch ships [`service`](#service),
and a repo's own live in `menubar/templates/<name>.yaml`.

```yaml
app: {name: wall, id: dev.example.wall.menubar, icon: rectangle.stack, interval: 5s}

use:
  daemon:
    service: {label: dev.example.wall.daemon, noun: Daemon}
  client:
    service: {label: dev.example.wall.client, noun: Client}

menu:
  - outlet
  - separator
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
```

```state both running
daemon: {agent: {running: true, pid: 4821}}
client: {agent: {running: true, pid: 4822}}
```

```state client stopped
daemon: {agent: {running: true, pid: 4821}}
client: {agent: {installed: true}}
```

Each entry takes exactly one key, the template, and its arguments. A use's name
is an object in any expression: its watches and its states are fields —
`daemon.agent.pid`, `client.stopped`.

### Writing a template

A template file takes `params`, `watch`, `state`, `status` and `menu`, and
nothing else. Templates do not nest.

```yaml template
params:
  url: ~            # ~ is required
  noun: Server      # anything else is the default

watch:
  health:
    http: ${url}

state:
  - down: "!self.health.ok"
  - up:

menu:
  default:
    - {text: "${noun} is down", when: self.down}
```

| Key | Takes |
|---|---|
| `params` | A mapping of names to defaults. `~` makes one required. |
| `watch`, `state` | The same as in a `menubar.yaml`, but the use's own. |
| `status`, `menu` | A mapping of [outlet](#outlets) names to lists. `default` is the default outlet. |

`${name}` fills in a parameter when perch builds the app, on each string value
after the template is parsed, so a value holding `: ` stays one string. `$${`
writes a literal `${`, and any other `$` is left alone, so shell text like
`$HOME` passes through; a malformed `${` is a build error. Inside `[ ]` or
`{ }`, quote it: `{http: "${url}"}`. `{{ }}` is untouched.

Inside a template, `self` is that use, and the only name it sees: a template
cannot come to depend on a file it was not written for. Each use gets its own
state list, so exactly one of its states holds whatever the file's do, and the
file's `state:` may name a use's states.

A status rule in a template needs a `when:`: it would otherwise land ahead of
the file's rules and shadow every one of them.

A repo template may not take the name of one perch ships, since a reader could
not tell which one runs.

### Outlets

`- outlet` in a file's `menu:` or `status:` is the default outlet, and so is
`- outlet: default`; `- outlet: controls` is a named one. Each use's fragments
land at the matching outlet, in `use:` order. A fragment whose outlet the file
does not declare goes to the default outlet, and with no default declared, that
is the end of `menu:` and the start of `status:`. A named outlet no use fills is
refused, which is what catches `control` written for `controls`.

### `service`

A LaunchAgent: whether it is installed, loaded and running, and the controls to
run it. It takes `label`, required, and `noun`, which defaults to `Service`.

| Part | Is |
|---|---|
| `self.agent` | A [`launchagent`](#launchagents) watch on `label`. |
| States | `uninstalled`, `stopped`, `idle`, `running`. |
| `status` default | A warning icon while uninstalled. |
| `menu` default | One readout line: `<noun>: running · pid 4821`. |
| `menu` controls | `Start <noun>`, `Restart <noun>` and `Stop <noun>`, each shown only when it can work. |

To change its labels, copy it into `menubar/templates/` under another name —
`perch schema -template -o menubar/templates/schema.json` gives an editor its
schema.

## Builtins

What perch does for a widget so its file does not have to spell it out.

### LaunchAgents

A `launchagent` watch reads a job launchd holds, and the `agent:` action starts,
stops or restarts it, so neither needs a hand-written `launchctl`. The label is
written once.

```yaml
app: {name: worker, id: dev.example.worker.menubar, icon: gearshape, interval: 10s}

watch:
  worker:
    launchagent: dev.example.worker

menu:
  - agent: worker.start
  - agent: worker.stop
  - agent: worker.restart
  - {text: Quit, quit: true}
```

The plist is assumed to be `~/Library/LaunchAgents/<label>.plist`; say `plist:`
beside `launchagent:` if it is somewhere else. The watch binds:

| Field | Is |
|---|---|
| `.installed` | The plist exists. |
| `.loaded` | launchd holds the label. |
| `.running` | The job has a process. |
| `.pid` | Its pid, or `0` when it is not running. |
| `.label` | The label. |
| `.plist` | The plist path, `~` expanded. |
| `.domain` | `gui/<your uid>`. |
| `.target` | `<domain>/<label>`, for a `launchctl` call perch does not write. |

There is no `.ok`, because loaded and running are different answers: a job that
has run and exited is loaded and not running.

`agent:` takes `<watch>.<verb>`. `worker.start` bootstraps the plist into your
GUI domain, `worker.stop` boots it out, and `worker.restart` boots it out, waits
up to two seconds for launchd to release the label, and bootstraps it again.
Two `run:` items cannot do that last one: `bootout` returns before the label is
free, and bootstrapping into that gap fails with `Input/output error`. The
domain and the plist path are worked out on the Mac the widget runs on, so the
same file works on every machine.

An `agent:` also works as a button on a [quit prompt](#quit), which is how an
app offers to stop its server on the way out. A button gets no verb guard, so
offer one only under a rule whose `when:` means it can work — `svc.loaded` for a
stop. [Start and stop a LaunchAgent](recipes/launchagent.md) is the whole
widget.

An `agent:` item shows only when its verb can work — Start while installed and
not loaded, Stop while loaded, Restart while installed — and a `when:` you write
is combined with that. With no `text:`, it is labeled Start, Stop or Restart.
The readout and all three controls are also the shipped [`service`](#service)
template.

### Handled without asking

| What | Where |
|---|---|
| A separator with nothing beside it is dropped. | [`menu`](#menu) |
| Every action re-polls the watches when it finishes. | [Actions](#actions) |
| `perch shape` writes a shape from a sample. | [`shape`](#shape) |
| A window gets a main menu, and ⌘Q closes it instead of quitting. | [`window`](#window) |
| `menubar/AppIcon.png` is rendered into every size the Dock asks for. | [The Dock tile](#the-dock-tile) |
| A quit prompt stands aside for logout, restart and shutdown. | [`quit`](#quit) |
| An `agent:` item with no `text:` is labeled Start, Stop or Restart, and shows only when its verb can work. | [LaunchAgents](#launchagents) |

## `status`

A list of rules deciding how the status item looks. **The first rule whose
`when:` holds wins.** A rule with no `when:` always matches, so it has to be
last; anything after it is refused rather than left unreachable.

| Key | Takes |
|---|---|
| `when` | A string: the condition. Omit it to match always. |
| `icon` | An SF Symbol name, or `{asset: <name>}`. Replaces `app.icon`. |
| `dim` | `true` or `false`. Draws the status item dimmed. |
| `badge` | A string: an expression, rendered as text beside the icon. |

One rule may set several of them — an `icon:` and `dim:` together, say.

## `window`

One WebKit window, opened from a menu item. One per app, matching one status
item per app — which is why nothing names it.

```yaml
app: {name: dash, id: dev.example.dash, icon: tray, interval: 5s}

window:
  url: http://localhost:4747/
  title: dash                # defaults to app.name
  size: [1280, 860]          # width, height; defaults to [1024, 768]
  zoom: {min: 0.5, max: 2.0, step: 0.1}   # omit for no zoom controls

menu:
  - {text: Open Dashboard, window: open}
  - {text: Quit, quit: true}
```

Only `url` is required.

Declaring a window also emits a main menu — App, File, Edit, View, Window, and
the status item's own menu mirrored under **Status**. Not optional: an
`.accessory` app owns no menu bar until it is `.regular`, and without the Edit
menu ⌘C does not work inside the window at all. ⌘Q closes the window; quitting
moves to ⌥⌘Q, because for a menu-bar-first app ⌘Q otherwise costs the status
item and its polling.

The mirrored menu is rebuilt from the last poll every time it opens, the same
way the status menu is, so the two cannot disagree about what is possible.

What the window does is fixed, because each one is a trap:

| Behavior | Why |
|---|---|
| Close hides, never destroys | Reopening keeps scroll position and whatever was expanded |
| A reopen reloads only if the last load failed | Held open across a server restart the page is dead HTML; reloading every time throws away the state the row above buys |
| A load is always fresh, never `reload()` | `reload()` does nothing when the first navigation failed and left no history entry |
| Presenting deminiaturizes first | `orderOut` does not clear the miniaturized flag, so a window hidden while minimized has no tile to come back from |
| The Dock icon follows the window | Raised before activating, or the activation lands on an app with no tile |

The frame is remembered across launches. Zoom is not — it resets to 1.0.

A Dock tile wants artwork: see [The Dock tile](#the-dock-tile).

## `menu`

A list of items, rebuilt from the last poll every time the menu opens, so the
menu never offers an action that cannot work.

The bare items are `separator` and [`outlet`](#outlets). Everything else is a mapping:

| Key | Takes |
|---|---|
| `text` | A string: the label. `{{ }}` holes interpolate expressions. |
| `when` | A string: a condition. The item appears only when it holds. |
| `each` | A string: an expression naming a list. The item repeats, with `it` bound to each element. |
| `menu` | A list of items, written the same way. |

Then at most one action, which is what activating the item does:

| Key | Takes |
|---|---|
| `run` | A list of at least one string: argv, never a shell. |
| `open` | A string: a URL, or a path with `~` expanded. |
| `post` | A mapping of `url` (a string) and `body` (any YAML, sent as JSON). |
| `agent` | A string, `<watch>.<verb>` — or `<use>.<watch>.<verb>`, or `self.<watch>.<verb>` inside a template — where the watch is a `launchagent` watch and `<verb>` is `start`, `stop` or `restart`. |
| `quit` | `true`. Ends the app. |
| `swift` | A string, `<Type>.<method>`: a static method in `menubar/Sources/`. |
| `window` | `open`, `close` or `reload`. Needs a [`window:`](#window) block. |

An item with a submenu takes no action: opening the submenu supersedes it, so it
could never run.

A separator with nothing beside it is dropped — leading, trailing, and every one
after the first in a run. Which items a poll leaves out is not knowable where
they are written, so guarding each separator by hand would mean repeating the
conditions of every item around it.

### Actions

Every action re-polls the watches as soon as it finishes, which is what makes a
Start item feel like it did something. A failure raises an alert naming what was
attempted.

`agent:` is the one action perch writes the command for: see
[LaunchAgents](#launchagents).

`swift:` calls hand-written Swift. It names a static method — dotted, so your
names cannot collide with the emitted ones — and perch emits the call without
checking it exists; `swiftc` does that when the app is built, in the same module
as `menubar/Sources/*.swift`. So `perch build` alone will not catch a misspelled
method, but `run` and `install` will. It re-polls afterward like every other
action, and raises no alert on failure: there is no exit status to inspect.

This is the menu-side half of [hooking the app from
Sources/](#hooking-the-app-from-sources), which is otherwise launch-only.

### `each`

`each:` names a list. The item — and its submenu — is repeated once per
element, with `it` bound to that element.

```yaml
app: {name: fleet, id: dev.example.fleet.menubar, icon: rectangle.3.group, interval: 5s}

watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true
    shape: {jobs: [{id: string, node: string, cmd: string}]}

menu:
  - each: fleet.data.jobs
    text: "{{it.node}} — {{it.cmd}}"
    menu:
      - {text: Logs, run: [onto, logs, "{{it.id}}"]}
      - {text: Kill, run: [onto, kill, "{{it.id}}"]}
  - {text: Quit, quit: true}
```

[Jobs, with a submenu each](recipes/jobs.md) is the whole widget.

## Expressions

Every `when:`, every `badge:`, and every `{{ }}` hole is a
[CEL](https://github.com/google/cel-spec) expression. perch parses it at build
time and lowers it to a plain Swift expression.

**Nothing evaluates CEL at runtime.** No interpreter ships in the app, and an
expression perch cannot lower is a build error rather than a widget that shows
nothing.

The supported subset is field selection and indexing, literals,
`== != < <= > >=`, `&& || !`, `in`, the ternary `? :`, and the functions
`size()`, `has()`, `string()`, `startsWith()` and `contains()`. Anything else
is refused with the offending expression quoted. perch does not claim to
implement CEL; it claims to reject what it has not implemented.

Four things the typing settles:

- An `int` compared against a `double` is refused rather than quietly widened.
- Both arms of a `? :` must have the same type.
- `in` against a declared list requires the element type to match.
- `has()` on a field a shape declares is a build-time constant, because the
  shape has already said the field is there.

There is no `null` literal.

## Files in a consuming repo

```
menubar.yaml           authored
menubar/Generated/     emitted; committed, and replaced whole on every build
menubar/Sources/       hand-written Swift; perch never reads or writes it
```

The directory boundary is the whole of the drift story: nothing outside
`Generated/` is ever overwritten, and nothing inside it is ever authored.
Ejecting is moving a file across that line and deleting the YAML that made it.

Generated Swift is committed, so the repo builds without perch installed.

### Hooking the app from `Sources/`

Everything in both directories compiles into one binary, so a hand-written file
can reach the emitted app. What it reaches is `Controller`, which declares
`NSApplicationDelegate` and implements none of its lifecycle methods — so an
extension supplies the witness and AppKit calls it:

```swift
// menubar/Sources/Alerts.swift
import AppKit

extension Controller {
    func applicationDidFinishLaunching(_ notification: Notification) {
        // your own timers, observers, notification center
    }
}
```

`applicationWillFinishLaunching`, `applicationDidFinishLaunching` and
`applicationWillTerminate` are reserved for you. perch will not start
implementing one: doing so would shadow yours with no error and no warning —
your file would still compile and simply stop running — so a test refuses it.

This is how a repo keeps behavior the schema deliberately refuses. Notification
delivery is the case it exists for: banners fire on a transition rather than on
a condition holding, and the cursor that makes them fire once is state no
`when:` can name.

The status item, the menu and the polling stay perch's. An extension that
redraws either is fighting a menu rebuilt from the last poll every time it
opens.

A menu item reaches hand-written code through [`swift:`](#actions).

## Icon assets

An SF Symbol is drawn as a template: macOS tints it for the menu bar's
appearance and for whether the bar is selected. Your own artwork is not
tinted, so it keeps the colors you drew — which is the point when the icon
says something a symbol cannot, such as how full something is.

Put `.png` files in `menubar/Icons` beside your spec and name one without its
extension. An asset name is letters, digits, dashes and underscores:

```yaml
app: {name: onto, id: dev.onto.menubar, icon: {asset: o-0}, interval: 5s}
watch:
  fleet:
    run: [onto, top, --json]
    json: true
    shape: {nodes: [{up: bool}]}
status:
  - when: "!fleet.ok"
    icon: {asset: o-problem}
  - icon: {asset: o-4}
menu:
  - {text: Quit, quit: true}
```

Every `.png` in that directory is copied into the bundle, and `perch build`
refuses a spec naming one that is not there — otherwise the app installs and
runs with no image at all, which reads as the poller failing.

Ship artwork at twice the size you want it drawn: it is scaled to the menu
bar's 18pt height with its aspect kept. Where an icon has to read against both
a light and a dark menu bar, add `<name>~dark.png` beside `<name>.png` and the
app picks per appearance.

### The Dock tile

`menubar/Icons` is the status item's artwork. A [`window:`](#window) app also
takes a Dock tile while its window is open, and that wants a different file:
`menubar/AppIcon.png`, one square PNG at 1024×1024. `perch install` renders it
into the ten sizes macOS asks for and names it in the bundle. There is no key
for it — the file being there is what turns it on.

Without one the tile is the generic blank application icon.
