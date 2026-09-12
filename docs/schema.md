# menubar.yaml

The reference for the file perch reads. It is for someone writing one; for why
the schema stops where it does, see [the design](superpowers/specs/2026-09-05-perch-design.md).

A document has five top-level keys — `app`, `watch`, `state`, `status` and
`menu`. Only `app` is required, though an app with no `menu` offers nothing but
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

## app

The first four keys are required.

| Key | Takes |
|---|---|
| `name` | A string, one path component: no `/`, no leading dot, not `.` or `..`. It names the `.app` under `~/Applications` and the executable inside it. |
| `id` | A string of letters, digits, dots, dashes and underscores, starting with a letter or digit — `dev.example.menubar`. launchd takes it as a label and `install` as a plist filename. |
| `icon` | An [SF Symbol](https://developer.apple.com/sf-symbols/) name, or `{asset: <name>}` where `<name>` is letters, digits, dashes and underscores. Shown when no `status:` rule overrides it. See [Icon assets](#icon-assets). |
| `interval` | A Go duration: a number and a unit, one or more times. The units are `ns`, `us`, `ms`, `s`, `m`, `h` — `5s`, `1m30s`. Must be positive. |
| `sign` | Optional. A string naming a code signing identity in your keychain. Omitted, the `.app` is signed ad-hoc — see [Signing](guide/install.md#signing-and-why-a-rebuild-can-lose-a-permission). |

## watch

A mapping of names to polled sources. They run concurrently, and they all
re-poll on the [`interval`](#app) the `app:` block sets. Their results are what
every expression in the document reads.

A name is letters, digits and underscores, starting with a letter or
underscore. `it` is taken — `each:` binds its element to that — and so are
CEL's own keywords.

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

A `launchagent` watch binds no `.ok`, because there are two answers and they
differ: `.loaded` is launchd holding the label, `.running` is the job having a
process. A job that has run and exited is loaded and not running. `.pid` is `0`
unless it is running. The last four are strings for `launchctl` calls perch does
not write for you: `.plist` with `~` expanded, `.domain` as `gui/<your uid>`,
and `.target` as `<domain>/<label>`.

A watch that fails sets `.ok` false and leaves `.data` null. It never takes the
app down, and the menu still opens.

### shape

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

## state

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
`it`. States and watches are named the same way in an expression, so a state
may not take a watch's name.

A condition names watches and nothing else. Naming another state is refused
rather than resolved: a state declared earlier is already excluded by the
ordering, so naming one could only ever be a constant, and a guard that looks
like a guard while contributing nothing is the failure this schema exists to
prevent.

## status

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

## menu

A list of items, rebuilt from the last poll every time the menu opens, so the
menu never offers an action that cannot work.

The only bare item is `separator`. Everything else is a mapping:

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
| `agent` | A string, `<watch>.<verb>`, where `<watch>` is a `launchagent` watch and `<verb>` is `start`, `stop` or `restart`. |
| `quit` | `true`. Ends the app. |

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

`agent:` is the one action perch writes the command for. `worker.start` boots
the LaunchAgent that `worker` watches into your GUI domain, `worker.stop` boots
it out, and `worker.restart` does both with a wait between — which is not two
`run:` items, because `bootout` returns before launchd has released the label.

```yaml
app: {name: worker, id: dev.example.worker.menubar, icon: gearshape, interval: 10s}

watch:
  worker:
    launchagent: dev.example.worker

menu:
  - {text: Start, when: "!worker.loaded", agent: worker.start}
  - {text: Stop, when: "worker.loaded", agent: worker.stop}
  - {text: Restart, when: "worker.installed", agent: worker.restart}
  - {text: Quit, quit: true}
```

[Start and stop a LaunchAgent](recipes/launchagent.md) is the whole widget.

### each

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
