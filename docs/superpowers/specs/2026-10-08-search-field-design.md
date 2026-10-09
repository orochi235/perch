# Search field

**Status: designed, not built.**

This is the design for whoever builds the `field:` menu item and whoever changes
it next. It answers: how a menu takes typed text and hands it to an action.

## The item

A `field:` item draws a search field in the menu. Its value is the placeholder.
Enter fires the item's action with the typed text bound as `query`.

```yaml
menu:
  - field: Search GitHub
    open: "https://github.com/search?q={{query}}"
  - field: Find job
    run: [onto, find, "{{query}}"]
```

| Key on a `field:` item | |
|---|---|
| `when` | Allowed, as on any item. |
| `each` | Allowed; `it` and `query` are both bound. |
| `run`, `open`, `post`, `swift` | Exactly one, required. |
| `agent`, `quit`, `window` | Refused: they cannot use the text. |
| `text`, `icon`, `menu` | Refused: the placeholder is the label, the field draws its own magnifier, and Enter cannot open a submenu. |

`query` anywhere other than a `field:` item's action is a build error, the same
as a misspelled field.

## Escaping

A `{{query}}` hole in an `open:` value is percent-encoded, so `a&b c` arrives as
one query parameter. Other holes in `open:` are left as they are, so an existing
`open: "{{it.url}}"` keeps working. `run:` argv and `post:` JSON get the text
raw; neither needs escaping.

## `swift:`

The method takes the text: `static func find(_ query: String)`. As with every
`swift:` action, `swiftc` checks the signature at `run` or `install`, not
`perch build`.

## Runtime

The item is an `NSMenuItem` whose view is an `NSSearchField`, focused when the
menu opens. Enter with text closes the menu, runs the action, and re-polls,
like every other action. Enter on an empty field does nothing; Esc closes the
menu. The text is not kept: the menu is rebuilt each time it opens.

Focus is the risk. A status item's menu tracks keystrokes itself, and handing
them to a text field in an item's view is known to be fiddly. Spike it before
the schema work.

## Testing

A golden fixture and a compile test, as for every item kind; a `render.go` test
per refusal above; a runtime test that puts text in the field, triggers Enter,
and checks the action received it.
