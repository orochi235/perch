# Declared states

**Status: built.** `state:` parses, validates, lowers and emits; the recipes
use it. This is the design, kept for whoever changes it next.

A widget is a state machine, and `menubar.yaml` had no way to say so. This
adds one: a `state:` block naming the states in order, each becoming a boolean
any expression can use.

## Why

Both apps perch was written to replace carry a state enum — `ServerState` in
one, `ServiceState` in the other — and the schema could not express the single
abstraction they independently arrived at. What it offered instead was a
`when:` on every item, so the state machine survived only as fragments in the
author's head.

That had a cost the recipes showed. `docs/recipes/launchagent.md` wrote
`!agent.ok && plist.ok` twice, once as an item's label guard and once as the
guard on Start. Nothing checked that the two agreed, or that the three labels
it offered covered every case, or that no two of them could show at once.

## The block

`state:` is an ordered list of one-key mappings. The first whose condition
holds wins. The last entry has no condition and is the fallback.

```yaml
state:
  - uninstalled: "!plist.ok"
  - stopped:     "!agent.ok"
  - running:
```

Ordering plus a mandatory fallback is what makes exactly one state hold at
every poll, with nothing to prove. The alternative — an unordered set of
conditions, checked for overlap and coverage — is not decidable over CEL in
general, and an atom-level approximation gives confidently wrong answers where
atoms correlate: `n == 0` and `n != 0` look like they can both hold if you
model them as independent booleans.

A state name follows the rule watch names follow — letters, digits and
underscores, starting with a letter or underscore, not `it` and not a CEL
keyword. States and watches are declared in the same CEL scope, so a state
taking a watch's name would shadow it: the collision is refused rather than
resolved.

## Using a state

A state is an ordinary boolean in every expression the document can write —
every `when:`, every `badge:`, every `{{ }}` hole:

```yaml
status:
  - {when: uninstalled, icon: exclamationmark.triangle}
  - {when: stopped,     dim: true}

menu:
  - {text: Not installed,   when: uninstalled}
  - {text: Needs attention, when: "uninstalled || stopped"}
```

There is no separate `state:` field on an item: it would be a second way to
say `when:`, buying only a check that CEL already performs on an undeclared
name.

A condition names watches and nothing else. Naming an earlier state is
refused, not allowed: the ordering has already excluded every earlier state,
so such a reference could only ever be a constant, and a guard that reads like
a guard while contributing nothing is the failure this schema exists to
prevent. Naming a later state is the same refusal, and needs no cycle check of
its own — the backend declares states one at a time, so a later name is simply
not bound yet.

## Where it runs

Not a desugaring pass. Substituting a state's text into an expression cannot
be done correctly: `uninstalled` inside a string literal, or as a shape's
field in `x.uninstalled`, must not be rewritten, and a pass working on text
cannot tell those apart from a reference.

Instead states reach the backend intact, as `States []State` on `Spec`, and
`celswift.Env` declares each as a bool. The type-checker resolves a reference
the same way it resolves a watch, and a misspelling comes back through the
diagnostic that already exists for an undeclared identifier.

`Env` had one binding it minted itself, `it` from `WithEach`. It gained
`WithState`, and `binding` gained a `local` flag so `Prefixed` can tell a loop
variable — which is already a name in scope — from everything it has to reach
through the results record.

State *n* resolves to every earlier state negated, and its own condition
asserted — so `when: stopped` and "the widget is in state stopped" are the
same claim, which they would not be if a state meant its raw condition:

    state 1     (c₁)
    state 2     !state1 && (c₂)
    state 3     !state1 && !state2 && (c₃)
    fallback    !state1 && !state2 && !state3

Each own condition is parenthesized. Without that, a state whose condition is
`a || b` comes apart under the leading negations, and still compiles — the
silent-wrong-answer failure this schema exists to prevent. The lowering
brackets its own output as well, so the emitted Swift carries a redundant
layer; relying on that instead would be an unwritten contract between two
packages.

They are emitted as properties of `Results` rather than as locals in the two
render functions:

```swift
extension Results {
    var state_uninstalled: Bool { (!(plist.ok)) }
    var state_stopped: Bool { !state_uninstalled && (!(agent.ok)) }
    var state_running: Bool { !state_uninstalled && !state_stopped }
}
```

Locals would read better and compute once per poll, but a state that only the
menu names would then be an unused binding in `renderFace`, and the typecheck
test fails on a warning. A property a function never reads is simply unread.
The cost is that a state named twice in one function is computed twice, which
is what every `when:` already does.

## Validation

`spec` checks only what it can see without parsing an expression, since
expressions are opaque strings until the backend:

- The fallback is required, and must be last. An entry after it is refused
  rather than left unreachable, matching the existing rule for `status:`.
- Names are unique, valid, not `it`, and do not take a watch's name.

Everything about what a condition names is the backend's, and falls out of
declaring states in order against an environment holding only the watches.

A declared state nothing references is allowed. The typo it might indicate is
already caught at the reference, by CEL.

## What changed

| File | Change |
|---|---|
| `internal/spec/state.go` | new: `State`, `parseStates`, `validateStates` |
| `internal/spec/spec.go` | `States` on `Spec`, `state` on `rawSpec`, the parse call |
| `internal/spec/validate.go` | call `validateStates` |
| `internal/celswift/env.go` | `WithState`, and `local` so `Prefixed` skips `it` |
| `internal/backend/swiftappkit/render.go` | `emitStates`, `stateProp`, states in `renderEnv` |
| `internal/schema/schema.go` | the `state` block |
| `docs/schema.md`, `internal/site/nav.go` | a `## state` section and its page |
| `docs/recipes/launchagent.md` | rewritten on states |

`drift_test` needed no new `sections` entry. Its map is per raw struct, and a
state entry's key is a name the author chose, so the block has no fixed key
set — the same hole `shape` sits in. Adding `state` to `rawSpec` did mean
adding it to the schema's top-level properties, which is the drift that map
does catch.

## Testing

- `internal/spec/state_test.go`: the parse, and a table of every refusal.
- `internal/backend/swiftappkit/state_test.go`: a condition naming another
  state in either direction, the shape of the emitted extension, and the
  parenthesization.
- A `states` golden, carried through `swiftc -typecheck` like every other. It
  reads a state from both render functions, from inside a larger expression,
  and from inside a string literal, where the name is text and stays text.
- `FuzzParse` covers the new block without changes.

## Then

`launchagent:` and `control:` — sugar expanding to the watch and the three
guarded items — were designed in conversation on 2026-09-11 and are not built.
They guard on a state rather than inventing a condition, which is why they
waited.
