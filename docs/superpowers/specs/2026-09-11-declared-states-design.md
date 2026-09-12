# Declared states

**Status: designed, not built.** Nothing in this document exists in the repo
yet.

A widget is a state machine, and `menubar.yaml` has no way to say so. This
adds one: a `state:` block naming the states in order, each becoming a boolean
any expression in the document can use. It is for whoever implements it.

## Why

Both apps perch was written to replace carry a state enum — `ServerState` in
one, `ServiceState` in the other — and the schema cannot express the single
abstraction they independently arrived at. What it offers instead is a `when:`
on every item, so the state machine survives only as fragments in the author's
head.

That has a cost the recipes show. `docs/recipes/launchagent.md` writes
`!agent.ok && plist.ok` twice, once as an item's label guard and once as the
guard on Start. Nothing checks that the two agree, or that the three labels it
offers cover every case, or that no two of them can show at once.

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
  - {text: Not installed, when: uninstalled}
  - {text: Not loaded,    when: stopped}
  - {text: Running,       when: running}
  - {text: Needs attention, when: "uninstalled || stopped"}
```

There is no separate `state:` field: it would be a second way to say `when:`,
buying only a check that CEL already performs on an undeclared name.

A state's own condition may name any state declared before it. Naming a later
one, or itself, is refused.

## Where it runs

Not a desugaring pass. Substituting a state's text into an expression cannot
be done correctly: `uninstalled` inside a string literal, or as a shape's
field in `x.uninstalled`, must not be rewritten, and a pass working on text
cannot tell those apart from a reference.

Instead states reach the backend intact, as a `States []State` on `Spec`, and
`celswift.Env` declares each one as a bool. The type-checker then resolves a
reference the same way it resolves a watch, and a misspelling comes back
through the diagnostic that already exists for an undeclared identifier.

`Env` today hardcodes the single binding `it`, added by `WithEach`. It gains a
way to declare a named boolean bound to a Swift expression, which is what both
`it` and a state need.

State *n* resolves to every earlier state negated, and its own condition
asserted — so `when: stopped` and "the machine is in state stopped" are the
same claim, which they would not be if a state meant its raw condition:

    state 1     (c₁)
    state 2     !state1 && (c₂)
    state 3     !state1 && !state2 && (c₃)
    fallback    !state1 && !state2 && !state3

Each own condition is parenthesized. Without that, a state whose condition is
`a && b` breaks apart under the leading negations, which parses, type-checks,
emits, and is wrong — the silent-wrong-answer failure this schema exists to
prevent.

The emitter writes these in order as `let` bindings at the top of `face(_:)`
and `menu(_:)`, so each state is computed once per poll and every reference is
a name:

```swift
let uninstalled = !(plist.ok)
let stopped     = !uninstalled && !(agent.ok)
let running     = !uninstalled && !stopped
```

That is the enum both original apps wrote by hand, and here it falls out of
the lowering rather than being an optimization on top of it.

## Validation

`spec` checks only what it can see without parsing an expression, since
expressions are opaque strings until the backend:

- The fallback is required, and must be last. An entry after it is refused
  rather than left unreachable, matching the existing rule for `status:`.
- Names are unique, valid, and do not take a watch's name.

The forward reference needs no check of its own. The backend resolves states
in order and declares each in the env as it goes, so a condition naming a
later state finds nothing declared and fails as an undeclared identifier —
with the position and the message that error already carries.

A declared state nothing references is allowed. The typo it might indicate is
already caught at the reference, by CEL.

## What has to change

| File | Change |
|---|---|
| `internal/spec/state.go` | new: the raw struct, the parse, the checks above |
| `internal/spec/spec.go` | `States` on `Spec`; parse the block |
| `internal/celswift/env.go` | declare a named bool bound to a Swift expression |
| `internal/backend/swiftappkit/render.go` | resolve each state, emit the `let` block, extend the env |
| `internal/schema/schema.go` | the `state` block |
| `internal/schema/drift_test.go` | a `sections` entry for the new struct |
| `docs/schema.md` | a `## state` section |
| `internal/site/nav.go` | that section's page — the site build fails without it |

The block decodes through its own tagged raw struct rather than being read off
`yaml.Node` by hand: that is what keeps `drift_test` able to see its keys, and
what gets it unknown-key rejection.

Nothing here needs a new command. The resolved conditions are visible in the
emitted Swift, which is committed.

## Testing

- Golden pairs in `swiftappkit`: a spec with states in, the `let` block and
  the expressions referencing it out. This is where the parenthesization and
  the ordering are legible, and the only place a wrong resolution shows.
- `swiftc -typecheck` over that golden, as every golden already gets.
- Table tests in `spec` for each refusal: missing fallback, fallback not last,
  duplicate name, collision with a watch.
- A `celswift` case for a condition naming a later state, asserting the
  undeclared-identifier diagnostic rather than silence.
- One `celswift` case per reference form — a bare state, a state under `!`, a
  state in `||`, and a state name appearing inside a string literal, which
  must survive untouched.
- `FuzzParse` covers the new block without changes.

## Then

`launchagent:` and `control:`, designed in conversation on 2026-09-11 and not
written down, guard their generated items with a state rather than inventing a
condition. That is why they wait: the sugar reads differently once states
exist, and designing it twice is the waste.
