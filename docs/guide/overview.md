# Give anything on your Mac a menu bar widget.

You write one file. perch turns it into a macOS menu bar app: what to poll, what
the icon shows, what the menu offers. It emits Swift, builds the `.app`, and
loads it as a LaunchAgent, so it is there again after a restart.

```
go install github.com/orochi235/perch/cmd/perch@latest
```

macOS only, and you need `swiftc` — the Xcode command line tools — since Swift
is what perch emits.

## What a file says

Four blocks: the app itself, the commands to watch, what the icon shows, and
what the menu offers. Every state below is one poll of the same file.

```yaml
app:
  name: fleet
  id: dev.example.fleet
  icon: rectangle.3.group
  interval: 5s

watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true
    shape:
      jobs: [{id: string, node: string}]

status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - when: "fleet.data.jobs.size() == 0"
    dim: true
  - badge: "fleet.data.jobs.size()"

menu:
  - text: "{{fleet.data.jobs.size()}} running"
  - separator
  - {text: Quit, quit: true}
```

```state busy
fleet:
  out:
    jobs:
      - {id: j1, node: studio}
      - {id: j2, node: studio}
      - {id: j3, node: mini}
```

```state idle
fleet:
  out: {jobs: []}
```

```state unreachable
fleet:
  code: 1
  err: "onto: no such host"
```

## Nothing is evaluated at runtime

Conditions and `{{ }}` holes are [CEL](https://github.com/google/cel-spec),
lowered to Swift when you build. No interpreter ships in the app, so a
misspelled field is a build error rather than a widget that quietly shows
nothing.

A `shape:` is what makes that checking worth having: it says what the command
prints, so `fleet.data.jbos` stops compiling instead of rendering as blank.
`perch shape` writes one for you from a sample run.

## What you run

| Command | |
|---|---|
| `perch run` | build, compile, and run in the foreground — the dev loop |
| `perch install` | bundle the app, write the plist, load the agent |
| `perch build` | emit Swift into `menubar/Generated/` |
| `perch shape` | run a command once and write its shape declaration |

Generated Swift lives in `menubar/Generated/` and is replaced whole on every
build. Anything you write by hand lives in `menubar/Sources/`, which perch never
reads or writes — so [ejecting](../schema.md#files-in-a-consuming-repo) is
moving a file across that line and deleting the YAML that made it.
