# Handoff — 2026-09-15

Branch `main` in `~/src/perch`: the templates work is merged, pushed and
released as [`v2.0.0`](https://github.com/orochi235/perch/releases/tag/v2.0.0).

## Landed

- **Templates, outlets, `self`, verb defaults and the shipped `service`
  template** — [design](specs/2026-09-14-templates-design.md),
  [plan](plans/2026-09-14-templates.md). Each plan task that landed differently
  from its text says so under its heading.
- **Module path is `github.com/orochi235/perch/v2`**, and the install lines say
  so. `v1.0.0` is tagged on `aa7cf95`.
- **Homebrew tap `orochi235/homebrew-tap`** (`~/src/homebrew-tap`):
  `brew install orochi235/tap/perch` builds the tagged tarball from source, and
  `brew test` and `brew audit --strict --online` pass. A release bumps `url` and
  `sha256` in `Formula/perch.rb`. homebrew-core would refuse perch until it has
  75 stars, 30 forks or 30 watchers (three times that if self-submitted).

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

**Spec 2 is not written**: code-backed built-ins, and a `services` built-in that
combines several services' state with one-click Start/Stop for all of them.

**30 stale Local Network rules** named `dev.perch.smoke.<pid>` remain in
`/Library/Preferences/com.apple.networkextension.plist`. The smoke test now uses
one fixed id, so no more accrue; removing the old ones needs root and has no
supported tool.

## What the docs tests do and don't prove

Every `yaml` fence in `docs/guide`, `docs/recipes`, `docs/schema.md` and the
README parses, emits and passes `swiftc -typecheck`, the site build compiles and
runs that Swift against each page's `state` fences, and every SF Symbol named
resolves against the running macOS. **Nothing runs the commands the recipes
poll**: their `shape:` declarations and stated outputs are hand-written, so a
recipe can be green while `brew outdated --json=v2` prints something else.
Checked by hand on 2026-09-15: brew, `onto top --once --json` and the `gh run
list` fields all match. The `http:` recipes name example URLs nothing serves.

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

- **Three perch binaries are installed.** `/opt/homebrew/bin/perch` (brew,
  `v2.0.0`) is first on PATH, then `~/.local/bin/perch`; `~/go/bin` is not on
  PATH. Testing a local build means running it by path.
- **macOS 27 needed the Xcode license re-accepted** before `swiftc` would run;
  every Swift-compiling test fails with exit 69 until it is.
