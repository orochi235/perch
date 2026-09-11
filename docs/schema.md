# menubar.yaml

The reference for the file perch reads. It is for someone writing one; for why
the schema stops where it does, see [the design](superpowers/specs/2026-09-05-perch-design.md).

A document has four top-level keys. Only `app` is required, though an app with
no `menu` offers nothing but its icon.

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

All four fields are required.

| Field | Takes |
|---|---|
| `name` | The bundle and executable name. One path component — it names a `.app` under `~/Applications`, so no `/` and no leading dot. |
| `id` | The bundle identifier, e.g. `dev.example.menubar`. launchd takes it as a label and `install` as a plist filename: letters, digits, dots, dashes, underscores. |
| `icon` | An [SF Symbol](https://developer.apple.com/sf-symbols/) name, shown when no `status:` rule overrides it. Or `{asset: <name>}` for your own artwork — see [Icon assets](#icon-assets). |
| `interval` | How often every watch re-polls, as a Go duration — `5s`, `1m30s`. Must be positive. |

## watch

A mapping of names to polled sources. They run concurrently, every `interval`,
and their results are what every expression in the document reads.

A name is letters, digits and underscores, starting with a letter or
underscore. `it` is taken — `each:` binds its element to that — and so are
CEL's own keywords.

Each watch is exactly one of three kinds.

```
run: [argv, …]     run a command; never a shell, so no pipes and no globbing
http: <url>        GET a URL
exists: <path>     test for a file
```

What a watch binds depends on its kind:

| Kind | Binds |
|---|---|
| `run` | `.ok` (exit status 0), `.code`, `.out`, `.err` |
| `http` | `.ok` (a 2xx), `.status`, `.out` |
| `exists` | `.ok` |

Adding `json: true` to a `run` or `http` watch decodes its output and binds
`.data` as well. An `exists` watch binds only `.ok`, so `json:` on one is an
error rather than a no-op.

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

## status

A list of rules deciding how the status item looks. **The first rule whose
`when:` holds wins.** A rule with no `when:` always matches, so it has to be
last; anything after it is refused rather than left unreachable.

| Field | Effect |
|---|---|
| `when` | The condition. Omit it to match always. |
| `icon` | An SF Symbol, or `{asset: <name>}`, replacing `app.icon`. |
| `dim` | Draw the item dimmed. |
| `badge` | An expression rendered as text beside the icon. |

One rule may set several of them — an `icon:` and `dim:` together, say.

## menu

A list of items, rebuilt from the last poll every time the menu opens, so the
menu never offers an action that cannot work.

The only bare item is `separator`. Everything else is a mapping:

| Field | Effect |
|---|---|
| `text` | The label. `{{ }}` holes interpolate expressions. |
| `when` | Show the item only when this holds. |
| `each` | Repeat the item over a list, binding each element to `it`. |
| `menu` | A submenu, written the same way. |
| `run` / `open` / `post` / `quit` | What activating the item does. |

An item takes at most one action, and an item with a submenu takes none:
opening the submenu supersedes the action, so it could never run.

### Actions

`run:` takes argv and never a shell. `open:` takes a URL or a path. `post:`
takes a `url` and a `body`, sent as JSON. `quit:` ends the app.

Every action re-polls the watches as soon as it finishes, which is what makes a
Start item feel like it did something. A non-zero exit raises an alert naming
the command.

Controlling a LaunchAgent needs no special support. Start, Stop and Restart are
three `run:` items with `when:` guards, so the menu offers only the one that can
work:

```yaml
app: {name: worker, id: dev.example.worker.menubar, icon: gearshape, interval: 10s}

watch:
  agent:
    run: [launchctl, print, gui/501/dev.example.worker]

menu:
  - text: Start
    when: "!agent.ok"
    run: [launchctl, bootstrap, gui/501, ~/Library/LaunchAgents/dev.example.worker.plist]
  - text: Stop
    when: "agent.ok"
    run: [launchctl, bootout, gui/501/dev.example.worker]
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

The supported subset is field selection and indexing, literals, `== != < <= >
>=`, `&& || !`, `in`, the ternary `? :`, and the functions `size()`, `has()`,
`string()`, `startsWith()` and `contains()`. Anything else is refused with the
offending expression quoted. perch does not claim to implement CEL; it claims
to reject what it has not implemented.

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
extension:

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
