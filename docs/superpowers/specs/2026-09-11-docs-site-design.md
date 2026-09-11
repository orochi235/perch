# perch docs site

**Status: being built.** This line changes when the site builds and deploys.

The design of perch's documentation site, for someone changing the site, adding
a recipe, or touching the emitted Swift the site's menu previews run on.

## What it is

A static site at `orochi235.github.io/perch`, built from the markdown in `docs/`
by `go run ./cmd/site`. The layout is comp C (`.comps/c.png`): a header, a left
sidebar, one column of prose. It follows the reader's light or dark setting;
dark is comp C's palette and light is comp A's. The Overview headline is "Give
anything on your Mac a menu bar widget." There is no search.

## Pages

| Sidebar | Pages | Source |
|---|---|---|
| Guide | Overview, Install, Your first app, Ejecting, Commands | `docs/guide/*.md` |
| menubar.yaml | the intro, then one page per `##` section | `docs/schema.md`, split at build |
| Recipes | one page each | `docs/recipes/*.md` |

The order and titles live in one table in `internal/site`. `schema.md` stays
one file, so it still reads whole on GitHub; the build splits it and rewrites
its `#anchor` links to the page each anchor landed on. A relative link to a
file the site does not publish goes to that file on GitHub.

In the Commands page, a fence whose info string is `help <command>` is replaced
by what `perch <command> -h` prints, so the flags shown cannot fall behind the
binary.

## Menu previews

A ` ```yaml ` fence followed directly by one or more ` ```state <name> ` fences
is drawn as the menu bar item and open menu it builds, with one tab per state.
A `yaml` fence with no `state` fence after it is only code.

A state fence is YAML mapping each watch to what one poll of it returned:

| Watch kind | Keys | `ok` is |
|---|---|---|
| `run` | `code` (default 0), `out`, `err` | `code == 0` |
| `http` | `status` (default 200), `out` | a 2xx |
| `exists` | a bare `true` or `false` | that value |

`out` may be written as a mapping or list, which is encoded as JSON. A watch a
state does not mention is one that never answered: `ok` false and every field
zero. A key a kind does not bind, or a watch the spec does not declare, is a
build error.

The preview is drawn by the Swift perch emits, not by a second evaluator — the
runtime coerces loosely (a missing field reads as `""`, `0` or `false`), and a
reimplementation would disagree exactly in the failing states recipes exist to
show. That needs the emitted app split in two:

- `Render.swift` holds the results types, one function per watch turning a poll
  outcome into its result, and `face(_:)` and `menu(_:)`: plain functions from
  the results to what the status item shows (icon, dim, badge) and the menu
  tree. A menu action is data, lowered when the menu is built, so an item runs
  what it showed.
- `main.swift` holds the controller: it polls, and draws `face` and `menu` with
  AppKit through the fixed runtime.

`swiftappkit` also emits a preview driver in place of the app's `main.swift`.
It reads the states as JSON on stdin, builds results through the same
per-watch functions, and prints `face` and `menu` for each. `internal/preview`
compiles and runs it.

An SF Symbol is drawn as an outline square with the symbol's name on hover:
Apple licenses the symbols for apps on its platforms, and a web page is not
one. An `{asset: …}` icon is drawn from its PNG in `docs/recipes/icons/`.

## Build and deploy

`go run ./cmd/site -o _site` builds the site; `-serve [::]:8080` also serves it.
The build needs macOS and `swiftc`, and fails without them rather than publish
pages with no previews.

`.github/workflows/site.yml` builds on every push to `main` and deploys to
GitHub Pages. Pages has to be enabled on the repo with "GitHub Actions" as its
source.

## Tests

- `internal/e2e/docs_test.go` covers `docs/guide/` and `docs/recipes/`: every
  `yaml` fence parses, emits, and type-checks.
- `internal/site` builds the whole site into a temp dir and fails on a state
  fence that does not render.
- `swiftappkit` drives `face` and `menu` through the preview driver with fixed
  states, which checks what a menu says and not only that it compiles.
