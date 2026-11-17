# TODO

## Generated Swift doesn't say which perch emitted it

Every `perch build` copies the runtime into `menubar/Generated/` beside the code it generates
(`internal/backend/swiftappkit/emit.go`), so the generator and runtime can't disagree inside one
build. What isn't versioned is the boundary with code perch doesn't own: hand-written Swift in
`menubar/Sources/` calling runtime types, and apps linking `PerchKit` through `Package.swift`. The
committed `Generated/` header names no perch version, so after an upgrade nothing tells a repo its
committed output came from an older generator, or that its `Sources/` used a runtime API that has
since changed. Stamping the version in the header means deciding how golden files
(`testdata/`) stay stable across releases; a clean-project build fixture against an installed perch
would cover the consumer side. (filed 2026-11-15 from automated review)

## The quit prompt's own path has no test

`Quit.should` (`internal/backend/swiftappkit/runtime/Actions.swift`) now replies with the chosen
button's result, so a failed action cancels the quit. The completion results it relies on are probed
headlessly in `runtime_test.go`, but the prompt runs an `NSAlert` modal, so nothing drives
`Quit.should` itself end to end: a regression that drops or inverts its `finish` reply would pass.
Covering it means a seam for the modal, like `Act.alert`, or a GUI test beside the window ones
behind `PERCH_WINDOW_TESTS`. (filed 2026-11-17)
