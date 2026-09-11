# Is my server up?

An `http:` watch GETs a URL every interval and binds whether it answered, the
status code, and — with `json: true` — what it returned. This is the widget for
a service you run locally and forget about.

```yaml
app:
  name: api
  id: dev.example.api.menubar
  icon: bolt.horizontal.circle
  interval: 15s

watch:
  api:
    http: http://127.0.0.1:8765/healthz
    json: true
    shape:
      version: string
      sessions: [{id: string, user: string}]

status:
  - when: "!api.ok"
    icon: exclamationmark.triangle
    dim: true
  - when: "api.data.sessions.size() == 0"
    dim: true
  - badge: "api.data.sessions.size()"

menu:
  - text: "version {{api.data.version}}"
    when: "api.ok"
  - text: "not answering — HTTP {{api.status}}"
    when: "!api.ok"
  - separator
  - each: api.data.sessions
    text: "{{it.user}}"
  - separator
  - {text: Open, open: "http://127.0.0.1:8765"}
  - {text: Quit, quit: true}
```

```state busy
api:
  out:
    version: 2.4.1
    sessions:
      - {id: s1, user: ana}
      - {id: s2, user: ben}
```

```state idle
api:
  out: {version: 2.4.1, sessions: []}
```

```state down
api:
  status: 503
```

`api.ok` is a 2xx. Anything else — a 503, a refused connection, a timeout —
leaves it false and `api.data` empty, and the menu still opens: it says what
went wrong instead of what is running.

`open:` takes a URL or a path. It is the only action that does not re-poll
afterwards, since nothing it does changes what the watch would find.
