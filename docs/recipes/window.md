# A local dashboard in a window

A server on localhost, its health in the menu bar, and its page in a window
the widget owns — rather than a browser tab that gets lost among forty others.

```yaml
app:
  name: dash
  id: dev.example.dash
  icon: tray
  interval: 5s

watch:
  health:
    http: http://localhost:8080/api/health

state:
  - down: "!health.ok && health.status == 0"
  - unhealthy: "!health.ok"
  - up:

window:
  url: http://localhost:8080/
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}

status:
  - {when: down, icon: tray, dim: true}
  - {when: unhealthy, icon: exclamationmark.triangle}
  - {icon: tray.full}

menu:
  - {text: "Server: not running", when: down}
  - {text: "Server: not responding", when: unhealthy}
  - {text: "Server: running", when: up}
  - separator
  - {text: Open Dashboard, window: open}
  - separator
  - {text: Quit, quit: true}
```

```state up
health:
  status: 200
```

```state unhealthy
health:
  status: 503
```

```state down
health:
  status: 0
```

`health.status` is `0` when nothing answered at all, which is what separates a
server that is down from one that is up and unwell. Nothing refused the
connection in the second case, so `.ok` alone cannot tell them apart.

The window is opened from the menu and closed with ⌘W. It keeps its scroll
position between openings, and takes a Dock tile only while it is up. Put a
1024×1024 `menubar/AppIcon.png` beside the spec, or that tile is the generic
blank application icon.
