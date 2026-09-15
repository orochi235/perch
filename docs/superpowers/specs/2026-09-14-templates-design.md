# Templates

**Status: designed, not built.** Nothing in this document exists in perch yet.
It is the first of two specs; the second — code-backed built-ins, and a
`services` built-in that combines several services' state and controls — is not
written. Both land as perch 2.0.

This is the design for whoever builds it and whoever changes it next. It
answers: how a `menubar.yaml` reuses a block of watches, states and menu items,
and why that does not reopen what the LaunchAgent design refused.

## Why

brainhouse, slopboard and onto each describe their LaunchAgents by hand: a
readout of whether an agent is installed, loaded and running, then some of
Start, Stop and Restart, each with its own `when:`. Every `agent:` Start among
them guards on the agent being installed and not loaded, and every Restart on a
condition that implies installed.

[The LaunchAgent design](2026-09-12-launchagent-watch-design.md) cut a
`control:` block for exactly this, for three reasons: its guards would bypass
`state:`, taking states as parameters is longer than the items it replaces, and
the first author who wants "Unload" instead of "Stop" abandons it. This design
answers each:

- A template carries **its own state list**, so its guards name its own states
  rather than raw conditions, and nothing is passed in.
- A template is **a YAML file**. An author who wants other labels copies it
  under a new name and edits it.
- Where its items land is decided by **outlets** the file declares, so the file
  keeps control of its own menu.

## Using a template

A top-level `use:` block, shaped like `watch:`: each entry names a use, and its
one key is the template.

```yaml
use:
  daemon:
    service: {label: tech.x.slopboard.daemon}
  client:
    service: {label: tech.x.slopboard.client, noun: Client}
```

## A template

A template is a YAML file taking `params`, `watch`, `state`, `status` and
`menu` — and nothing else. `app`, `window` and `use` are refused, so templates
do not nest.

```yaml
# service.yaml, shipped with perch
params:
  label: ~          # ~ is required
  noun: Service     # anything else is the default

watch:
  agent: {launchagent: "${label}"}

state:
  - uninstalled: "!self.agent.installed"
  - stopped: "!self.agent.loaded"
  - idle: "!self.agent.running"
  - running:

status:
  default:
    - {when: self.uninstalled, icon: exclamationmark.triangle}

menu:
  default:
    - {text: "${noun}: not installed", when: self.uninstalled}
    - {text: "${noun}: not loaded", when: self.stopped}
    - {text: "${noun}: loaded, not running", when: self.idle}
    - {text: "${noun}: running · pid {{self.agent.pid}}", when: self.running}
  controls:
    - {text: "Start ${noun}", agent: self.agent.start}
    - {text: "Restart ${noun}", agent: self.agent.restart}
    - {text: "Stop ${noun}", agent: self.agent.stop}
```

Shipped templates live in `internal/templates/` and are embedded in the binary.
A repo's own live in `menubar/templates/<name>.yaml`. A repo template taking a
shipped template's name is refused rather than preferred: a reader of
`menubar.yaml` could not tell which one runs, and a fix to the shipped one would
silently never arrive.

Nothing outside the repo is read, so a build answers the same on a laptop and on
CI.

## Parameters

`${name}` is filled in after the template is parsed as YAML, on each string
value, so a value holding `: ` or a newline stays one string and cannot change
the file's structure. Substitution fills only `${name}`: `$${` writes a literal
`${`, and any other `$` is left alone, so shell text like `$HOME` or `$$` passes
through. A malformed `${` (unclosed, empty, or not a name) is a build error.

`{{ }}` is untouched: it is an expression the app evaluates on every poll, and
`${}` is text perch fills in once, at build.

`text/template` was rejected because it shares `{{ }}` with perch's holes and
works on raw text, so a substituted value can break indentation.

## State

Each use gets its own ordered list: exactly one of its states holds at every
poll, by the same rule as `state:`. slopboard is why: in one shared list, the
client's `stopped` could only hold while the daemon was running, since an
earlier match excludes everything after it.

The file's own `state:` may name a use's states:

```yaml
state:
  - down: "daemon.stopped"
  - wedged: "!wall.ok"
  - up:
```

That is the one loosening of the rule that a state condition names only
watches. The rule exists because naming a state from the same list can only
ever be a constant; a use's state is from another list, so it is not.

## Names

Inside a template, `self` is that use — the choice Kubernetes' CEL rules make
for the object a rule is on, and the counterpart of the `it` that `each:` binds.

Outside, a use's name is an object with the same fields: its watches as records
and its states as booleans — `daemon.agent.pid`, `client.running`. A use's name
shares the namespace of watches and states. A template sees only `self`: naming
a file's watch or state from inside one is refused.

## Outlets

In a template, `status:` and `menu:` are mappings from an outlet name to a list.
In a file, `- outlet` marks the default outlet and `- outlet: controls` a named
one.

```yaml
menu:
  - outlet                  # every use's default fragment
  - separator
  - text: Open Dashboard
    open: http://localhost:8765/
  - outlet: controls        # every use's controls
  - separator
  - {text: Quit, quit: true}
```

Each use's fragments land at the matching outlet, in `use:` order. A fragment
whose outlet the file does not declare goes to the default outlet. With no
default declared, it is the end of `menu:` and the start of `status:` — the
start, because a rule after the file's `when:`-less rule could never match.

## Verb defaults

These apply to every `agent:` item, in or out of a template:

- With no `text:`, the label is Start, Stop or Restart.
- The item shows only when its verb can work. Start needs the agent installed
  and not loaded, Stop needs it loaded, and Restart needs it installed.
- An author's `when:` is combined with that condition, so an item can never
  offer a verb that would fail.

This changes files that exist: an `agent:` item with no `when:` shows today
whether or not its verb can work. That, and the module path below, is why this
is 2.0.

## How it's built

- **Parse.** `spec.Parse` takes a template lookup that `project.Load` supplies.
  A template's sections go through the same `parseWatches`, `parseStates`,
  `parseStatus` and `parseMenu` as the file's, so refusals read the same inside
  and out. Outlets are resolved here and never reach the backend.
- **The spec.** `Spec` gains `Uses`, each a name, a template and its own
  watches and states. `StatusRule` and `Item` gain a `Scope`: the use they came
  from, or empty. Verb defaults are applied here: an `agent:` item carries its
  guard in its own `Guard` field, beside its author's `when:` rather than
  spliced into it; the backend lowers and joins the two, so an author's
  mistake quotes only what they wrote.
- **`agent:` takes a path.** `self.agent.start` and `daemon.agent.restart` are a
  path to a `launchagent` watch and a verb; `server.start` still parses. Quit
  buttons take the same form.
- **Expressions.** `celswift.Env` binds a use's name to an object type of its
  watches and states, and binds `self` to the same while lowering anything
  scoped. Member access already lowers to Swift member access.
- **Swift.** Each use is a struct nested in `Results` —
  `results.daemon.agent.pid`, `results.daemon.state_running`. A use's watches
  poll beside the file's; its states are computed before the file's, in order.
- **Around it.** `perch schema` writes a schema for template files too. The
  module path becomes `github.com/orochi235/perch/v2`, since Go's `@latest`
  ignores a `v2.0.0` tag on a module path without `/v2`; the README's install
  line changes with it. The LaunchAgent design's section retiring `control:`
  gets a pointer here.

## Errors

All are build errors naming the use and the template file.

| Case | The error |
|---|---|
| Unknown template | lists the shipped and repo templates |
| A repo template named like a shipped one | names both files |
| A missing required parameter, an undeclared argument, an unknown or malformed `${…}` | names it |
| `app:`, `window:` or `use:` in a template | templates do not nest |
| A template naming a file's watch or state | a template sees only `self` |
| `self` outside a template | it has nothing to refer to |
| A use named like a watch, a state, `self` or `it` | an expression could not tell them apart |
| A status rule with no `when:` in a template | it would land ahead of the file's rules and shadow them |
| `- outlet: <name>` that no use fills | catches `control` written for `controls` |
| One outlet declared twice in a list | its fragments would have two homes |

No runtime failure is new: a failed `agent:` raises the alert it does today.

## What it does not do

- **Templates using templates.** Nothing needs it yet, and it would make a
  use's scope a chain rather than one object.
- **Imports from a path or URL.** Sharing between repos is copying the file, or
  proposing it as a shipped template.
- **Combined state and one-click controls for several services.** That is
  spec 2, because running several `launchctl` operations in order, and deciding
  what a failure part way through means, is code rather than a template.

## Testing

- **`internal/spec`:** a refusal case per row of the error table; substitution
  (`$${`, a value holding `: `, `{{ }}` untouched); outlet placement (default at
  the end of `menu:` and the start of `status:`, named, fall-through, `use:`
  order).
- **`internal/celswift`:** `self` and a use's name lower to the right member
  access, and `it` and `self` both resolve inside a template's `each:`.
- **Goldens:** two `service` uses and a file `state:` naming them, carried
  through `swiftc -typecheck` like every golden. Copies of brainhouse's and
  onto's menus rewritten on `service` sit in `testdata`, as evidence the
  template covers files that exist; those repos are not changed.
- **Previews:** a state fence accepts nested values —
  `daemon: {agent: {running: true}}` — so a docs page can show a use's states.
- **Docs:** `docs/schema.md` gains a `use` chapter, the Builtins page lists
  `service`, and the LaunchAgent recipe is rewritten on it. The site build
  already fails when a schema section goes unpublished.
