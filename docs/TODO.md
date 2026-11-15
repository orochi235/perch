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
