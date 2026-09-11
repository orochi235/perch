# Commands

Every command takes `-C <dir>` to work somewhere other than the current
directory. All of them read the `menubar.yaml` at the root of that directory.

## perch run

Build, compile and run in the foreground. This is the loop you work in: quit
the app, edit the file, run it again.

```help run
```

## perch build

Emit Swift into `menubar/Generated/` and stop. The directory is replaced whole,
and what it holds is meant to be committed, so the repo builds without perch
installed.

```help build
```

## perch install

Build, compile, bundle the `.app` into `~/Applications`, write the LaunchAgent
plist, and load it. Run over a copy that is already running, it replaces and
restarts it.

```help install
```

## perch uninstall

Unload the agent, and remove both the plist and the app.

```help uninstall
```

## perch shape

Run a command once, or read JSON files you already have, and print the `shape:`
declaration for what came back. Both flags repeat, and several samples union.

```help shape
```

## perch schema

Write the JSON Schema for `menubar.yaml`, which is what gives an editor
completion and inline errors.

```help schema
```
