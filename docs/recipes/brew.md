# Homebrew updates

What is out of date, and a way to upgrade any one of them without a terminal.
`brew outdated --json=v2` prints two lists, so this widget declares both and
counts the one worth a badge.

```yaml
app:
  name: brew
  id: dev.example.brew.menubar
  icon: shippingbox
  interval: 30m

watch:
  outdated:
    run: [brew, outdated, --json=v2]
    json: true
    shape:
      formulae: [{name: string, current_version: string}]
      casks: [{name: string, current_version: string}]

status:
  - when: "!outdated.ok"
    icon: exclamationmark.triangle
  - when: "outdated.data.formulae.size() == 0 && outdated.data.casks.size() == 0"
    dim: true
  - badge: "outdated.data.formulae.size()"

menu:
  - text: "{{outdated.data.formulae.size()}} formulae · {{outdated.data.casks.size()}} casks"
  - separator
  - each: outdated.data.formulae
    text: "{{it.name}} → {{it.current_version}}"
    menu:
      - {text: Upgrade, run: [brew, upgrade, "{{it.name}}"]}
  - each: outdated.data.casks
    text: "{{it.name}} → {{it.current_version}}"
    menu:
      - {text: Upgrade, run: [brew, upgrade, --cask, "{{it.name}}"]}
  - separator
  - {text: Upgrade everything, run: [brew, upgrade]}
  - {text: Quit, quit: true}
```

```state outdated
outdated:
  out:
    formulae:
      - {name: ripgrep, current_version: 14.1.1}
      - {name: sqlite, current_version: 3.46.0}
    casks:
      - {name: iterm2, current_version: 3.5.5}
```

```state current
outdated:
  out: {formulae: [], casks: []}
```

```state no brew
outdated:
  code: 127
  err: "env: brew: No such file or directory"
```

Thirty minutes is the interval because `brew outdated` is not free and nothing
here changes minute to minute. Every watch in a file shares one interval, which
is a reason to keep a widget about one thing.

An upgrade takes as long as it takes, and the menu is rebuilt from the last poll
each time it opens, so the item disappears on the poll after the upgrade
finishes rather than the moment you click it.
