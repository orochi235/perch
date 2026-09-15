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

The label is written once. [`service`](../schema.md#service) watches it, keeps
its own four states, and puts a readout at the default outlet and Start Worker,
Restart Worker and Stop Worker at `controls` — each shown only when it can
work, so the menu never offers Start on a job launchd already holds.

The states are fields of the use: `worker.running`, `worker.stopped`. The watch
is `worker.agent`, and for a `launchctl` call perch does not write,
`worker.agent.target` is `gui/<your uid>/dev.example.worker`:

```
  - text: Why is it running?
    when: "worker.agent.loaded"
    run: [launchctl, blame, "{{worker.agent.target}}"]
```

**Loaded and running are different questions.** `launchctl print` succeeds for
any label launchd is holding, including a job that has already run and exited,
so a widget reading its exit status says Running about a job with no process.
[LaunchAgents](../schema.md#launchagents) has the rest.

Every action re-polls as soon as it finishes, which is what makes Start feel
like it did something: the menu that reopens says running.
