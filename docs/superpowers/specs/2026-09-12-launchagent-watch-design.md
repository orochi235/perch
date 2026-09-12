# The launchagent watch

**Status: built.** `launchagent:` parses, validates and emits; `agent:` is the
fifth action verb; the recipe and the reference are written on both. This is
the design, kept for whoever changes it next.

perch learns one third-party tool's vocabulary. This says which parts, and
where the line is.

## Why

The schema could not express a correct `launchctl` invocation at all. `run:`
argv reaches `/usr/bin/env` verbatim — no shell, so no `~` expansion — and
nothing bound the user's id. The recipe that shipped for a week hardcoded
`gui/501` and passed a literal `~/Library/LaunchAgents/…` to `bootstrap`, which
launchd does not expand. Both are wrong on any machine that is not the one the
recipe was written on.

The second reason is narrower and worse. `launchctl print` exits 0 for any
label launchd is holding, including a job that has run and exited, so `agent.ok`
means **loaded**, not **running**. The recipe called that state `running` and
was believed.

Neither is fixable by being careful, which is the bar for perch knowing about
launchd at all.

## The kind

A fourth `watch:` kind, beside `run`, `http` and `exists`:

```yaml
watch:
  worker:
    launchagent: dev.example.worker
    plist: ~/opt/worker/agent.plist   # optional; defaults from the label
```

One `launchctl print` and one stat answer everything it binds, so it is one
polled source like the others rather than a top-level block:

| Binds | |
|---|---|
| `.installed` | the plist is there |
| `.loaded` | launchd holds the label |
| `.running` | the job has a process |
| `.pid` | `0` unless running |
| `.label`, `.plist`, `.domain`, `.target` | strings for `launchctl` calls perch does not write |

It binds no `.ok`. Two answers would fit and they differ, so the name is
refused rather than resolved — through `Type.Hints`, a map of field name to why
it is absent, which the "no field" diagnostic in `celswift` consults. It exists
for this one case; a second use should make it carry its weight or go.

The last four strings are the escape hatch. `launchctl blame`, `enable`,
`kickstart -p` and everything else perch has no verb for stay writable as
ordinary `run:` items, and correct, because `.domain` reads `getuid()` on the
machine the widget runs on.

## The verb

`agent: worker.restart` — a fifth action beside `run`, `open`, `post` and
`quit`. A watch name holds no dot, so the one separator is unambiguous. The
verbs are `start`, `stop` and `restart`; naming anything else is refused with
the three quoted, and naming a watch of another kind is refused by saying which
kind it is.

`start` is `bootstrap`, `stop` is `bootout`. The legacy `load`/`unload` pair is
unreachable: `load -w` also writes launchd's disabled database, which is a
second effect nobody asking for Start is asking for.

`restart` is the one the schema could not express. `bootout` returns before
launchd has released the label, so bootstrapping into that gap fails with
`Input/output error` — as two menu items it races, and `kickstart -k`, which
the old recipe used instead, cannot start a job that is not loaded. The action
boots out, polls `print` until the label is gone, then bootstraps: the loop
`internal/install/launchctl.go` already runs when perch reloads its own widget.

## What it does not do

`launchagent:` contributes no states. States are one ordered list with a
mandatory fallback, so a block that injects three of them is contributing to a
machine it cannot see the rest of — the ordering that decides which state wins
becomes invisible, and a widget wanting `running` *and* `unhealthy` from a
health watch has nowhere to put the second. It also caps a document at one
agent: `running` for which one? The author writes the `state:` block, over the
predicates this binds.

The same reasoning retires `control:`, the menu-item sugar designed alongside
this. Guarding its three items on a raw condition is what `state:` was added to
stop; taking the states as parameters is longer than the three items it
replaces; and the first author who wants "Unload" instead of "Stop" abandons it
and writes all three by hand.

Three more were designed and cut:

- **Checking the plist's own `Label` against the watch.** It can only fire when
  the plist is on the build machine, so `perch build` would answer differently
  on a laptop and on CI.
- **Mapping launchd's error codes to sentences.** The codes are not documented
  well enough to explain confidently, and a wrong explanation is worse than
  `launchctl`'s own stderr, which the alert already shows.
- **Stat first, and skip the `launchctl` spawn when the plist is absent.** An
  agent loaded from a plist that was since deleted would then read as not
  loaded. That trades a right answer for a saved subprocess.

## What changed

| File | Change |
|---|---|
| `internal/spec/watch.go` | `WatchLaunchAgent`, `Label`, `Plist`, the default |
| `internal/spec/menu.go` | `ActionAgent`, `AgentVerb`, `parseAgentAction` |
| `internal/spec/validate.go` | `checkAgentTarget`; `json:` and the label refused |
| `internal/spec/name.go` | `checkLabel` |
| `internal/spec/shape.go` | `Type.Hints` |
| `internal/celswift/env.go` | the eight fields, and the `ok` hint |
| `internal/celswift/lower.go` | the hint reaches the "no field" diagnostic |
| `internal/backend/swiftappkit/` | the poll, the result struct with no `ok`, `.agent(…)` |
| `runtime/Runtime.swift` | `LaunchAgentOutcome`, `Launchd`, `Watcher.launchAgent`, `Act.agent` |
| `runtime/Preview.swift`, `internal/preview` | `installed`/`loaded`/`running`/`pid` in a state fence |
| `internal/schema/schema.go` | `launchagent`, `plist`, `agent` |
| `docs/schema.md`, `docs/recipes/launchagent.md` | rewritten on the kind |

## Testing

Two of these run against real launchd, in `internal/backend/swiftappkit`,
because nothing else can tell the difference the kind exists for:

- `TestLaunchAgentWatchSeparatesLoadedFromRunning` bootstraps a `sleep 120` and
  a job with `RunAtLoad` false, and asserts the second reads loaded and not
  running. A golden, a typecheck and a compile all stay green while `running`
  means `loaded`.
- `TestAgentRestartFromRunningAndFromStopped` restarts an agent that is up and
  one that has been booted out, and checks the pid changed both times.

Both skip outside an Aqua session. The rest is the usual shape: a refusal table
in `internal/spec`, a `launchagent` golden carried through `swiftc -typecheck`,
and the recipe's own four states rendered by compiling what perch emits for it.
