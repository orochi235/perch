# perch

Generate a macOS menu bar app from a YAML file.

You write `menubar.yaml` — what to poll, what the icon should say, what the
menu offers. perch emits Swift, builds it into a `.app`, and loads it as a
LaunchAgent.

```yaml
app:
  name: onto
  id: dev.onto.menubar
  icon: rectangle.3.group
  interval: 5s

watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true

status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - badge: "fleet.data.jobs.size()"

menu:
  - text: "{{fleet.data.jobs.size()}} running"
  - separator
  - {text: Quit, quit: true}
```

## Install

```
go install github.com/orochi235/perch/cmd/perch@latest
```

macOS only, and you need `swiftc` — the Xcode command line tools — since Swift
is what perch emits.

## Use

Write a `menubar.yaml` at the root of a repo, then:

```
perch run           build, compile, run in the foreground (the dev loop)
perch install       build, compile, bundle, write the plist, bootstrap
```

`perch install` puts the `.app` in `~/Applications` and a LaunchAgent in
`~/Library/LaunchAgents`, so it comes back at login. `perch uninstall` takes
both away.

The rest:

```
perch build         emit Swift into menubar/Generated/
perch shape         run a command once and write its shape declaration
perch schema        write the JSON Schema for menubar.yaml
```

Every command takes `-C <dir>` to work somewhere other than the current
directory.

## How it decides things

Conditions and `{{ }}` holes are [CEL](https://github.com/google/cel-spec),
lowered to Swift when you build. Nothing evaluates them at runtime, so a
misspelled field is a build error rather than a widget that shows nothing.

Emitted files live in `menubar/Generated/` and are replaced on every build.
Hand-written Swift lives in `menubar/Sources/`, which perch never reads or
writes — so ejecting is moving a file across that line and deleting the YAML
that made it. Generated Swift is committed, so the repo builds without perch
installed.

## Docs

[`docs/schema.md`](docs/schema.md) is the reference for `menubar.yaml`: every
key, what a watch binds, and the CEL subset expressions may use.

[`docs/superpowers/specs/2026-09-05-perch-design.md`](docs/superpowers/specs/2026-09-05-perch-design.md)
is the design — what the schema deliberately cannot say, and why the generator
is Go while the output is Swift.

## Developing

`go test ./...` needs the Swift toolchain: the tests that matter most compile
what perch emits, and one installs a LaunchAgent and watches the app stay up.
The fuzz targets only replay their seed corpus under `go test`; `bin/fuzz` runs
each of them for real.

## License

MIT.
