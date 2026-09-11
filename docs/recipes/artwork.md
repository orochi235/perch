# Your own artwork

An SF Symbol is drawn as a template: macOS tints it, so it can say only what a
shape can say. When the thing your widget reports is a color — pass, fail,
running — the icon has to keep the colors you drew it with, and that means
artwork.

```yaml
app:
  name: ci
  id: dev.example.ci.menubar
  icon: {asset: ci-unknown}
  interval: 2m

watch:
  ci:
    run: [gh, run, list, --limit, "1",
          --json, "status,conclusion,displayTitle,url"]
    json: true

status:
  - when: "!ci.ok"
    icon: {asset: ci-unknown}
  - when: 'ci.data[0].status != "completed"'
    icon: {asset: ci-running}
  - when: 'ci.data[0].conclusion == "success"'
    icon: {asset: ci-pass}
  - icon: {asset: ci-fail}

menu:
  - text: "{{ci.data[0].displayTitle}}"
    when: "ci.ok"
  - text: "gh could not say"
    when: "!ci.ok"
  - separator
  - text: Open the run
    when: "ci.ok"
    open: "{{ci.data[0].url}}"
  - {text: Quit, quit: true}
```

```state passing
ci:
  out:
    - status: completed
      conclusion: success
      displayTitle: emit compiling Swift
      url: https://github.com/orochi235/perch/actions/runs/1
```

```state failing
ci:
  out:
    - status: completed
      conclusion: failure
      displayTitle: fix five ways a file failed to parse
      url: https://github.com/orochi235/perch/actions/runs/2
```

```state running
ci:
  out:
    - status: in_progress
      conclusion: ""
      displayTitle: document the schema
      url: https://github.com/orochi235/perch/actions/runs/3
```

```state no gh
ci:
  code: 127
  err: "env: gh: No such file or directory"
```

Put the four PNGs in `menubar/Icons` beside the file and name one without its
extension. They are flat colors that read against a light or a dark menu bar,
so each needs only one file; [Icon assets](../schema.md#icon-assets) covers what
to do when one does not, and what size to draw at.

This watch declares no `shape:`, which is why `ci.data[0]` is written with an
index and compared to strings: `gh` returns a list, and a shape describes a
mapping. Without one, a misspelled field is not a build error here — the price
of reading something whose shape you have not written down.
