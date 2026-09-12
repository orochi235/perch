# Handoff — 2026-09-12

Branch `main` in `~/src/perch`, clean and pushed through `9317741`.

## Landed this session

- **`launchagent:` watch kind and `agent:` action** —
  [design](specs/2026-09-12-launchagent-watch-design.md).
- **Separators with nothing beside them are dropped** in `renderMenu`.
- **`perch install` signs the bundle**; `app.sign:` names a keychain identity.

## Not done, in rough order of value

**brainhouse is still hand-written Swift.** `~/src/brainhouse/menubar/main.swift`
is the app perch was written to replace, and it is the last one not on perch. It
controls `com.brainhouse` (a LaunchAgent) and carries the `ServiceState` enum
that motivated `state:`, so it is now expressible. Porting it is the real test of
whether the schema is finished.

**A guarded separator is still impossible.** The collapse pass handles a divider
next to nothing, which was the whole observed problem, but `- separator` is a
bare scalar and `itemFields` has no key for it, so `{separator: true, when: …}`
is not accepted. Only add this if a case turns up that collapsing does not cover.

**`app.sign:` is machine-specific and committed.** `onto` and `slopboard` both
name `perch local signing`, an identity that exists only in this Mac's login
keychain. Anyone else building those repos gets a failed install with a clear
codesign error. Acceptable for two private repos; wrong if perch ever has
outside users. No fallback was added on purpose — see the design note in the
commit.

**The TCC claim is reasoned, not measured end to end.** What is measured: a
source edit moves the cdhash while the designated requirement holds. What is not:
that a specific Local Network grant actually survived, because `TCC.db` needs
Full Disk Access and the menu-bar screenshot came back with an unreadable color
cast. If it matters, grant Full Disk Access and read `access` in
`~/Library/Application Support/com.apple.TCC/TCC.db`.

## Traps

- **`~/.local/bin/perch` shadows `~/go/bin/perch`**, and `~/go/bin` is not on
  PATH. `go install` alone changes nothing; copy the binary across.
- `onto` and `slopboard` both carry unrelated uncommitted work in other
  directories. Only `menubar.yaml` and `menubar/Generated/` were touched.
