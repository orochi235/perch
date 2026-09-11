# Uncommitted changes

A watch does not have to return JSON. `git status --porcelain` prints nothing at
all when a tree is clean, which is the whole condition this widget needs.

```yaml
app:
  name: repo
  id: dev.example.repo.menubar
  icon: checkmark.circle
  interval: 30s

watch:
  tree:
    run: [git, -C, /Users/you/src/project, status, --porcelain]
  stash:
    exists: /Users/you/src/project/.git/refs/stash

status:
  - when: "!tree.ok"
    icon: questionmark.circle
  - when: 'tree.out != ""'
    icon: pencil.circle
  - dim: true

menu:
  - text: Clean
    when: 'tree.ok && tree.out == ""'
  - text: Uncommitted changes
    when: 'tree.out != ""'
  - text: Not a repository
    when: "!tree.ok"
  - text: Something is stashed
    when: "stash.ok"
  - separator
  - {text: Open in Finder, open: /Users/you/src/project}
  - {text: Quit, quit: true}
```

```state clean
tree:
  out: ""
```

```state dirty
tree:
  out: " M internal/site/build.go\n?? notes.md\n"
stash: true
```

```state gone
tree:
  code: 128
  err: "fatal: not a git repository"
```

A `run:` watch is argv and never a shell, so there is no `cd` and no `~`: `git`
gets `-C` with an absolute path. `exists:` does expand `~`, since it only ever
takes a path.

`tree.out` is exactly what the command printed, trailing newline and all. It is
worth comparing against `""` or asking `contains()` rather than putting it in a
menu item, which would carry the newline into the title.
