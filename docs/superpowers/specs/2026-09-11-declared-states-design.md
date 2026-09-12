# Declared states

**Status: designed, not built.** Nothing in this document exists in the repo
yet.

A widget is a state machine, and `menubar.yaml` has no way to say so. This
adds one: a `state:` block naming the states, and a `state:` field usable
anywhere `when:` is. It is for whoever implements it.

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
keyword. States and watches share one namespace, so a state may not take a
watch's name.

## Using a state

Anywhere `when:` is accepted, `state:` is accepted instead:

```yaml
status:
  - {state: uninstalled, icon: exclamationmark.triangle}
  - {state: stopped,     dim: true}

menu:
  - {text: Not installed, state: uninstalled}
  - {text: Not loaded,    state: stopped}
  - {text: Running,       state: running}
```

An item takes at most one of `when:` and `state:`.

`state:` naming something undeclared is a build error listing the states that
exist. That is the part a raw expression cannot offer: a misspelled `when:`
is an error only if it happens to break typing, while a misspelled state is
always one.

## Where it runs

A desugaring pass in `internal/spec`, after `resolveAliases` and before
`decodeStrict`, rewriting the YAML node tree. `state:` fields are replaced by
the `when:` they stand for and the `state:` block is removed, so nothing
downstream — `Spec`, the CEL environment, `internal/backend/swiftappkit` —
learns the concept exists. The expansion is then checked by exactly the rules
that check hand-written YAML.

State *n* lowers to every earlier condition negated, and its own asserted:

    state 1     (c₁)
    state 2     !(c₁) && (c₂)
    state 3     !(c₁) && !(c₂) && (c₃)
    fallback    !(c₁) && !(c₂) && !(c₃)

Every condition is parenthesized on substitution. Dropping the parentheses
turns `!stopped` into `!!agent.ok && plist.ok`, which parses, type-checks,
emits, and is wrong — the silent-wrong-answer failure this schema exists to
prevent.

The repeated conditions this produces are the emitter's problem to collapse
later: computing the state once into a Swift enum and switching on it is an
optimization behind an unchanged schema, and is what the two original apps
wrote by hand. It is not a prerequisite.

## Validation

Beyond what the expanded YAML already gets:

- The fallback is required, and must be last. An entry after it is refused
  rather than left unreachable, matching the existing rule for `status:`.
- Names are unique, valid, and do not collide with a watch name.
- Every `state:` names a declared state.
- A declared state nothing references is allowed. The typo it might indicate
  is already caught at the reference.

## Seeing the expansion

`perch flat` reads the spec, expands every state, and prints the result
without emitting. Reading the expansion is also how an author learns what to
write by hand when the block does not fit. (`expand` says it more plainly;
`flat` is shorter to type. Either name, not both.)

## What has to change

| File | Change |
|---|---|
| `internal/spec/state.go` | new: the raw struct, the pass, its checks |
| `internal/spec/spec.go` | call the pass after `resolveAliases` |
| `internal/spec/menu.go`, `status.go` | accept `state:`, refuse it beside `when:` |
| `internal/schema/schema.go` | the `state` block and the `state` field |
| `internal/schema/drift_test.go` | a `sections` entry for the new struct |
| `cmd/perch/main.go` | `perch flat` |
| `docs/schema.md` | a `## state` section |
| `internal/site/nav.go` | that section's page — the site build fails without it |

The sugar keys decode through their own tagged raw structs rather than being
read off `yaml.Node` by hand. That is what keeps `drift_test` able to see them
and what gets them unknown-key rejection.

## Testing

- Golden pairs, sugared YAML in and flat YAML out. This is the cheapest place
  to catch a bad expansion, and the only place the parenthesization is legible.
- One golden carried through to Swift, so an expansion that parses but emits
  nothing sensible fails too.
- Table tests for each refusal: missing fallback, fallback not last, duplicate
  name, collision with a watch, `state:` beside `when:`, undeclared reference.
- `FuzzParse` covers the new pass without changes.

## Then

`launchagent:` and `control:`, designed in conversation on 2026-09-11 and not
written down, guard their generated items with a state rather than inventing a
condition. That is why they wait: the sugar reads differently once states
exist, and designing it twice is the waste.
