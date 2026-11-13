package swiftappkit

// lookDoc wears every part of an item's look: a header and rows from one each:,
// an enabled: that overrides an action, a shortcut, and menus without a gutter.
const lookDoc = `
app: {name: looks, id: dev.example.looks.menubar, icon: circle, interval: 3s, gutter: false}
watch:
  m:
    run: [echo, "{}"]
    json: true
    shape:
      rows: [{text: string, header: bool, depth: int, note: string}]
menu:
  - each: m.data.rows
    text: "{{it.text}}"
    header: it.header
    indent: "it.header ? 0 : it.depth"
    weight: "{{it.header ? 'bold' : ''}}"
    subtitle: "{{it.note}}"
    size: "it.header ? 15 : 0"
    separator: it.header
    menu:
      - {text: Open, when: "!it.header", run: [echo, open]}
  - {text: Reload, key: cmd+r, enabled: "m.ok", run: [echo, reload]}
  - {text: Back, key: cmd+left, run: [echo, back]}
  - {text: Help, key: f1, run: [echo, help]}
  - {text: Inert but live, enabled: true}
  - text: More
    gutter: false
    menu:
      - {text: Inside, run: [echo, inside]}
  - {text: Quit, quit: true}
`
