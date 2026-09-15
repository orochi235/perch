# Handoff — 2026-09-15

Branch `templates` in `~/src/perch`: built, reviewed and green, **not merged, not
pushed, not tagged**. `main` is at `09ee104` plus the spec and plan commits.

## Landed on the branch

- **Templates, outlets, `self`, verb defaults and the shipped `service`
  template** — [design](specs/2026-09-14-templates-design.md),
  [plan](plans/2026-09-14-templates.md). Each plan task that landed differently
  from its text says so under its heading.
- **Module path is `github.com/orochi235/perch/v2`**, and the install lines say
  so. `v1.0.0` is tagged on `aa7cf95`.

## Decisions made during the build, not in the original design

- **Parameters are braces-only.** Only `${name}` is filled; `$${` writes a
  literal `${`; every other `$` (`$HOME`, `$$`, `$1`) is left alone. `os.Expand`
  was dropped because it rewrote shell text and ate malformed `${`.
- **A verb's guard is `Item.Guard`**, lowered beside the author's `when:` in
  `render.go`, not spliced into it — so a `//` comment can't swallow it and an
  author's error quotes only what they wrote.
- **`service`'s controls name the service** (`Restart Daemon`), so two uses in
  one menu can be told apart.
- **No JSON Schema validator dependency.** The template schema is checked by
  structural tests and one test tying `service.yaml` to its `launchagent`
  pattern.

## Next

**Merge and release are waiting on a decision.** Tagging `v2.0.0` should carry
notes on what existing files see differently:
- an `agent:` item now hides when its verb can't work, and one with no `text:`
  gets Start, Stop or Restart;
- `init`, `Type`, `Protocol` and `self` are refused as names, and a file state
  `x` beside a watch or use named `state_x` is refused (it never compiled);
- install is `go install github.com/orochi235/perch/v2/cmd/perch@latest`.

**Spec 2 is not written**: code-backed built-ins, and a `services` built-in that
combines several services' state with one-click Start/Stop for all of them.

**Homebrew**: a draft source-build formula is in the session scratchpad only,
and a tap needs a home (`orochi235/homebrew-hued` or a new
`orochi235/homebrew-tap`). homebrew-core would refuse perch today (repo under 30
days, no stars). The test install upgraded brew's Go to 1.27.1.

**30 stale Local Network rules** named `dev.perch.smoke.<pid>` remain in
`/Library/Preferences/com.apple.networkextension.plist`. The smoke test now uses
one fixed id, so no more accrue; removing the old ones needs root and has no
supported tool.

## Small follow-ups

- The schema's state and shape field name rules don't list CEL's reserved
  words, which the parser refuses.
- The template schema accepts `${…}` only in `launchagent`, so an editor flags
  `json: ${decode}`; and it allows outlet marks in a template's nested
  submenus, which the parser refuses.

## Older, still open

- **reviewplex** (`~/src/pw/reviewplex`, Point Wild) is the last widget not on
  perch.
- **A guarded separator** (`{separator: true, when: …}`) is still not accepted;
  add only if collapsing doesn't cover a real case.
- **`app.sign:` in onto and slopboard** names an identity that exists only in
  this Mac's keychain.

## Traps

- **`~/.local/bin/perch` shadows `~/go/bin/perch`**, and `~/go/bin` is not on
  PATH. `go install` alone changes nothing; copy the binary across.
- **macOS 27 needed the Xcode license re-accepted** before `swiftc` would run;
  every Swift-compiling test fails with exit 69 until it is.
