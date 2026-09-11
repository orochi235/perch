# Install

```
go install github.com/orochi235/perch/cmd/perch@latest
```

perch runs on macOS and needs the Xcode command line tools, because it compiles
the Swift it emits:

```
xcode-select --install
```

Nothing else is required at runtime. The app perch builds is a plain `.app`
with no dependency on perch itself, so a machine that runs your widget never
needs the generator.

## Where an installed widget goes

`perch install` writes two things:

| Path | What |
|---|---|
| `~/Applications/<name>.app` | the built app |
| `~/Library/LaunchAgents/<id>.plist` | the LaunchAgent that starts it at login |

`perch uninstall` removes both and unloads the agent. Re-running `perch install`
over a running copy replaces it and restarts it.

## Completion and errors in your editor

`perch schema` writes the JSON Schema for `menubar.yaml`:

```
perch schema -o menubar.schema.json
```

Point your editor at it with a comment on the first line of the file, and a
misspelled key is underlined as you type rather than at build:

```yaml
# yaml-language-server: $schema=./menubar.schema.json
app: {name: fleet, id: dev.example.fleet, icon: circle, interval: 5s}
menu:
  - {text: Quit, quit: true}
```
