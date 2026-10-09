# Default Quit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status: planned, not built.**

**Goal:** When no menu item has `quit: true`, perch appends a separator and `Quit`.

**Architecture:** One function in `internal/spec` runs after `placeMenu` and `applyVerbDefaults` in `ParseWith`. It walks the placed menu, submenus included, and appends the two items when it finds no `ActionQuit`. The emitted Swift, the preview, and the site all read `Spec.Menu`, so nothing downstream changes.

**Tech Stack:** Go.

**Spec:** `docs/superpowers/specs/2026-10-08-default-quit-design.md`

## Global Constraints

- A guarded (`when:`) quit item counts as the author's own.
- `app.quit:` buttons don't count.
- There is no opt-out key.
- Fixtures that already end in Quit must emit byte-identical Swift.

## Review Focus

- **An empty `menu:`**: the result is just `Quit`, because the runtime's `tidy` drops the leading separator. Pinned in Task 1.
- **A quit item only inside a template's outlet**: it counts, because the check runs after placement. Pinned in Task 1.
- **A quit inside an `each:` item's submenu**: it counts, even though the list might be empty at run time. That's the author's choice, the same as a guarded quit. Pinned in Task 1.
- **Errors that quote item paths**: the added items need a path of their own, so an error never points at a line the author didn't write. Pinned in Task 1 (`Path()` is `menu (added by perch)`).
- **Goldens for fixtures without a Quit item** change, and nothing else does. Checked in Task 2.

---

### Task 1: Append the item in the spec

**Files:**
- Create: `internal/spec/quit_item.go`
- Modify: `internal/spec/spec.go` (call it after `applyVerbDefaults`)
- Test: `internal/spec/quit_item_test.go`

- [ ] **Step 1: Write the failing tests**: one per case in the spec's Testing section, plus the empty menu. Each one parses a document and checks whether the last two items are a separator and `{Text: "Quit", Action.Kind: ActionQuit}`.
- [ ] **Step 2: Run them and confirm they fail.** Run: `go test ./internal/spec/ -run DefaultQuit`
- [ ] **Step 3: Implement**

```go
package spec

// addQuit appends a Quit item when nothing in the menu quits. A status item
// has no Dock icon and no ⌘Q, so without one it cannot be closed from the UI.
func addQuit(menu []Item) []Item {
	if quits(menu) {
		return menu
	}
	const path = "menu (added by perch)"
	return append(menu, Item{Separator: true, path: path}, Item{Text: "Quit", Action: Action{Kind: ActionQuit}, path: path})
}

func quits(items []Item) bool {
	for _, it := range items {
		if it.Action.Kind == ActionQuit || quits(it.Menu) {
			return true
		}
	}
	return false
}
```

In `ParseWith`: `s.Menu = addQuit(s.Menu)` right after `applyVerbDefaults(s.Menu)`.

- [ ] **Step 4: Run `go test ./internal/spec/`.** Expected: PASS. Fix any existing test that counted menu items.
- [ ] **Step 5: Commit.**

### Task 2: Goldens, previews, docs

- [ ] **Step 1:** Run `go test ./internal/backend/swiftappkit/ -run TestGolden -update`, then `git diff --stat testdata`. Only fixtures with no Quit item should change. Read each diff.
- [ ] **Step 2:** In `docs/schema.md`'s `## menu`, say that perch adds the item and when. Remove the trailing `{text: Quit, quit: true}` from examples where it's the only quit, except in the `app.quit` example. Do the same in `docs/guide/` and `docs/recipes/`.
- [ ] **Step 3:** Run `go test ./internal/preview/ ./internal/site/ ./internal/e2e/ ./internal/backend/swiftappkit/`. Fix tests that count menu rows.
- [ ] **Step 4:** Delete the spec and this plan. Move the HANDOFF entry to **Landed**. Commit.
