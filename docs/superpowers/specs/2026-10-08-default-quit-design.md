# Default Quit item

**Status: designed, not built.**

This is the design for whoever builds the default Quit item and whoever changes
it next. It answers: when perch adds a Quit item the author did not write, and
why there is no way to turn that off.

## Why

A status item has no Dock icon and no ⌘Q. Without a Quit item, the only way to
get rid of one is `launchctl` or Activity Monitor. Every example in the docs, and
every `menubar.yaml` under `~/src` as of 2026-10-08, ends with the same
hand-written `{text: Quit, quit: true}`.

## The rule

If no item in the menu carries `quit: true`, perch appends a separator and
`{text: Quit, quit: true}` to the end of it.

- "In the menu" means after uses are placed: submenus and items a template
  supplies count.
- A `quit:` item guarded by `when:` counts. Its author has decided when Quit
  shows, and perch does not add a second one for the times it doesn't.
- A quit button inside `app.quit:` is not a menu item and does not count.

The item is appended to the spec's menu after placement, before anything reads
it. That way the emitted Swift, the preview driver, and the docs site all see the
same menu, with no second place deciding whether Quit exists.

`app.quit:` rules apply to it as they do to a hand-written Quit item: they hang
off terminating, not off the item.

## No opt-out

There is no key to suppress the item. Nobody has asked for a status item that
cannot be quit. Adding a key later breaks nobody; removing one would.

If one is ever added, it is not `quit: never`, because perch cannot keep that
promise. A window app's Dock tile still quits it, and logout and `kill` still end
it. It would be named for what it controls, whether perch adds the item (e.g.
`app.quit_item: false`), and refused beside any `quit: true` item.

## Docs

`docs/schema.md`'s `menu` section says the item is added and when. Examples
whose last item is a plain `{text: Quit, quit: true}` lose that line, so the docs
show the default. The `app.quit` example keeps its own, since it is about quit.
The recipes' menu previews then draw the added item, because they are rendered
from the same spec.

## Testing

- A spec test for each case above: no quit item (appended, after a separator),
  a top-level quit, a quit in a submenu, a guarded quit, a quit only in a
  template's outlet, and a quit only in an `app.quit:` button (appended).
- The goldens regenerate. Fixtures that already end in Quit must not change.
