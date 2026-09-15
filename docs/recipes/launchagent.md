# Start and stop a LaunchAgent

A background job you own, with a light for what it is doing and the controls to
run it.

```yaml
app:
  name: worker
  id: dev.example.worker.menubar
  icon: gearshape
  interval: 10s

use:
  worker:
    service: {label: dev.example.worker, noun: Worker}

status:
  - outlet
  - {when: "worker.stopped || worker.idle", dim: true}

menu:
  - outlet
  - separator
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
```

```state running
worker: {agent: {running: true, pid: 4821}}
```

```state loaded, not running
worker: {agent: {loaded: true}}
```

```state not loaded
worker: {agent: {installed: true}}
```

```state not installed
worker: {agent: {installed: false}}
```

[`service`](../schema.md#service) watches the label and fills two
[outlets](../schema.md#outlets): `- outlet` gets a warning icon while the job is
not installed and a line saying what it is doing, and `- outlet: controls` gets
Start Worker, Restart Worker and Stop Worker, each shown only when it can work.

Its states are `worker.uninstalled`, `worker.stopped`, `worker.idle` and
`worker.running`, and its watch is `worker.agent`, so a `launchctl` call perch
does not write can name `worker.agent.target`:

```
  - text: Why is it running?
    when: "worker.agent.loaded"
    run: [launchctl, blame, "{{worker.agent.target}}"]
```

`idle` is a job launchd holds with no process, such as one that ran and exited.
[LaunchAgents](../schema.md#launchagents) has the rest.

Every action re-polls as soon as it finishes, which is what makes Start Worker
feel like it did something: the menu that reopens says Worker: running.
