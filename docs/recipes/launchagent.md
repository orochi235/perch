# Start and stop a LaunchAgent

A background job you own, with a light for whether it is loaded and three items
to control it. Controlling launchd needs no special support: Start, Stop and
Restart are ordinary `run:` items with `when:` guards, so the menu only ever
offers the one that can work.

```yaml
app:
  name: worker
  id: dev.example.worker.menubar
  icon: gearshape
  interval: 10s

watch:
  agent:
    run: [launchctl, print, gui/501/dev.example.worker]
  plist:
    exists: ~/Library/LaunchAgents/dev.example.worker.plist

status:
  - when: "!plist.ok"
    icon: exclamationmark.triangle
  - when: "!agent.ok"
    dim: true

menu:
  - text: Running
    when: "agent.ok"
  - text: Not loaded
    when: "!agent.ok && plist.ok"
  - text: Not installed
    when: "!plist.ok"
  - separator
  - text: Start
    when: "!agent.ok && plist.ok"
    run: [launchctl, bootstrap, gui/501, ~/Library/LaunchAgents/dev.example.worker.plist]
  - text: Stop
    when: "agent.ok"
    run: [launchctl, bootout, gui/501/dev.example.worker]
  - text: Restart
    when: "agent.ok"
    run: [launchctl, kickstart, -k, gui/501/dev.example.worker]
  - separator
  - {text: Quit, quit: true}
```

```state running
agent:
  out: "dev.example.worker = { state = running }"
plist: true
```

```state stopped
agent:
  code: 113
  err: "Could not find service in domain"
plist: true
```

```state not installed
agent:
  code: 113
plist: false
```

`gui/501` is your login session: `501` is the first user account a Mac creates,
and `id -u` prints yours.

Both watches say only whether they worked. `agent` runs `launchctl print`, which
fails when the service is not loaded, so `agent.ok` is "loaded"; `plist` tests
for the file, so `plist.ok` is "installed". Neither needs `json:` — nothing here
reads what they printed.

Every action re-polls as soon as it finishes, which is what makes Start feel
like it did something: the menu that reopens says Running.
