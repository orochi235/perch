# Start and stop a LaunchAgent

A background job you own, with a light for what it is doing and three items to
control it.

```yaml
app:
  name: worker
  id: dev.example.worker.menubar
  icon: gearshape
  interval: 10s

watch:
  worker:
    launchagent: dev.example.worker

state:
  - uninstalled: "!worker.installed"
  - stopped: "!worker.loaded"
  - idle: "!worker.running"
  - running:

status:
  - when: uninstalled
    icon: exclamationmark.triangle
  - when: "stopped || idle"
    dim: true

menu:
  - text: "Running · pid {{worker.pid}}"
    when: running
  - text: Loaded, not running
    when: idle
  - text: Not loaded
    when: stopped
  - text: Not installed
    when: uninstalled
  - separator
  - text: Start
    when: stopped
    agent: worker.start
  - text: Stop
    when: "running || idle"
    agent: worker.stop
  - text: Restart
    when: "!uninstalled"
    agent: worker.restart
  - separator
  - {text: Quit, quit: true}
```

```state running
worker: {running: true, pid: 4821}
```

```state loaded, not running
worker: {loaded: true}
```

```state not loaded
worker: {installed: true}
```

```state not installed
worker: {installed: false}
```

The label is written once. The plist is assumed to be
`~/Library/LaunchAgents/dev.example.worker.plist`; say `plist:` beside
`launchagent:` if it is somewhere else.

**Loaded and running are different questions.** `launchctl print` succeeds for
any label launchd is holding, including a job that has already run and exited —
so a widget that reads its exit status says Running about a job with no process.
`worker.loaded` is launchd knowing the label; `worker.running` is the job having
one. That is why a launchagent watch binds no `.ok`: it would have to pick one
of the two answers for you.

Start, Stop and Restart write their own `launchctl` — the domain (`gui/` and
your user id) and the expanded plist path are worked out on the machine the
widget runs on, so the file is the same on every Mac.

Restart is offered whenever the agent is installed, because it does not need
the job to be up: it boots out, waits for launchd to let go of the label, and
bootstraps again. Written by hand it would be two menu items, and those race —
`bootout` returns before the label is free, and bootstrapping into that gap
fails with `Input/output error`.

For launchctl calls perch does not write, `worker.target` is
`gui/<your uid>/dev.example.worker` and `worker.domain` is the domain alone:

```
  - text: Why is it running?
    when: "worker.loaded"
    run: [launchctl, blame, "{{worker.target}}"]
```

Every action re-polls as soon as it finishes, which is what makes Start feel
like it did something: the menu that reopens says Running.
