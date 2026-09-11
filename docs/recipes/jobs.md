# Jobs, with a submenu each

A list that changes length. `each:` repeats one item over a list and binds the
element to `it`, and the item's submenu is repeated with it — so every job
carries its own actions, aimed at its own id.

```yaml
app:
  name: fleet
  id: dev.example.fleet.menubar
  icon: rectangle.3.group
  interval: 5s

watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true
    shape:
      nodes: [{name: string, up: bool}]
      jobs: [{id: string, node: string, cmd: string}]

status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - when: "fleet.data.jobs.size() == 0"
    dim: true
  - badge: "fleet.data.jobs.size()"

menu:
  - text: "{{fleet.data.nodes.size()}} nodes · {{fleet.data.jobs.size()}} running"
  - separator
  - each: fleet.data.jobs
    text: "{{it.node}} — {{it.cmd}}"
    menu:
      - {text: Logs, run: [onto, logs, "{{it.id}}"]}
      - {text: Kill, run: [onto, kill, "{{it.id}}"]}
  - separator
  - {text: Quit, quit: true}
```

```state busy
fleet:
  out:
    nodes:
      - {name: studio, up: true}
      - {name: mini, up: true}
    jobs:
      - {id: j1, node: studio, cmd: render frames 1-240}
      - {id: j2, node: studio, cmd: encode master}
      - {id: j3, node: mini, cmd: thumbnails}
```

```state quiet
fleet:
  out:
    nodes: [{name: studio, up: true}]
    jobs: []
```

```state unreachable
fleet:
  code: 1
  err: "onto: dial tcp: connection refused"
```

The `shape:` is what makes the three expressions worth writing. Without it,
`fleet.data.jbos.size()` compiles and the badge is silently 0; with it, the
build stops and names the field. `perch shape --from 'onto top --once --json'`
writes it from one real run.

An empty list is not an error, so the quiet state still opens a menu — it dims
the icon instead, which is the rule ahead of the badge.
