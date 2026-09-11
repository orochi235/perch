# Your first app

A widget that watches GitHub's own status page, so you can build it without
anything installed but perch. It takes four steps, and each one runs.

## 1. An app with nothing in it

Make a directory, and put this in `menubar.yaml`:

```yaml
app:
  name: ghstatus
  id: dev.example.ghstatus
  icon: cloud
  interval: 60s

menu:
  - {text: Quit, quit: true}
```

Then:

```
perch run
```

A cloud appears in the menu bar with one item in it. `perch run` builds,
compiles and runs in the foreground; quit it from the menu or with `^C`.

## 2. Watch something

A watch is a command to run, a URL to GET, or a file to test for. GitHub
publishes its status as JSON, so this one is a URL:

```yaml
app:
  name: ghstatus
  id: dev.example.ghstatus
  icon: cloud
  interval: 60s

watch:
  gh:
    http: https://www.githubstatus.com/api/v2/status.json
    json: true

menu:
  - text: "{{gh.data.status.description}}"
  - separator
  - {text: Quit, quit: true}
```

`json: true` decodes what came back and binds it as `gh.data`. Run it again and
the menu says "All Systems Operational".

## 3. Say something with the icon

`status:` is a list of rules, and the first one whose `when:` holds wins. A rule
with no `when:` always matches, so it goes last.

```yaml
app:
  name: ghstatus
  id: dev.example.ghstatus
  icon: cloud
  interval: 60s

watch:
  gh:
    http: https://www.githubstatus.com/api/v2/status.json
    json: true

status:
  - when: "!gh.ok"
    icon: exclamationmark.triangle
  - when: 'gh.data.status.indicator != "none"'
    icon: exclamationmark.cloud
  - dim: true

menu:
  - text: "{{gh.data.status.description}}"
  - separator
  - {text: Open the status page, open: "https://www.githubstatus.com"}
  - {text: Quit, quit: true}
```

The last rule dims the icon when everything is fine, which is what you want from
a widget you are not meant to look at.

## 4. Declare what the command returns

Nothing above knows what `gh.data` holds, so `gh.data.status.indicater` would
compile and then show nothing. A `shape:` fixes that, and perch writes it from a
sample:

```
curl -s https://www.githubstatus.com/api/v2/status.json > sample.json
perch shape --sample sample.json
```

Paste what it prints under the watch:

```yaml
app:
  name: ghstatus
  id: dev.example.ghstatus
  icon: cloud
  interval: 60s

watch:
  gh:
    http: https://www.githubstatus.com/api/v2/status.json
    json: true
    shape:
      status: {indicator: string, description: string}

status:
  - when: "!gh.ok"
    icon: exclamationmark.triangle
  - when: 'gh.data.status.indicator != "none"'
    icon: exclamationmark.cloud
  - dim: true

menu:
  - text: "{{gh.data.status.description}}"
  - separator
  - {text: Open the status page, open: "https://www.githubstatus.com"}
  - {text: Quit, quit: true}
```

```state operational
gh:
  out:
    status: {indicator: none, description: All Systems Operational}
```

```state incident
gh:
  out:
    status: {indicator: major, description: Partial outage of Actions}
```

```state unreachable
gh:
  status: 503
```

Now a typo in a field name is a build error. An inferred shape describes only
the samples it saw, so a healthy sample leaves out every field that appears when
something is wrong — take it as a starting point, not an answer.

## 5. Keep it

```
perch install
```

The app goes to `~/Applications`, the LaunchAgent to `~/Library/LaunchAgents`,
and it comes back at login. `perch uninstall` takes both away.
