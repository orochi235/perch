# Install

```
go install github.com/orochi235/perch/cmd/perch@latest
```

perch runs on macOS and needs the Xcode command line tools, because it compiles
the Swift it emits:

```
xcode-select --install
```

Nothing else is required at runtime. The app perch builds is a plain `.app`
with no dependency on perch itself, so a machine that runs your widget never
needs the generator.

## Where an installed widget goes

`perch install` writes two things:

| Path | What |
|---|---|
| `~/Applications/<name>.app` | the built app |
| `~/Library/LaunchAgents/<id>.plist` | the LaunchAgent that starts it at login |

`perch uninstall` removes both and unloads the agent. Re-running `perch install`
over a running copy replaces it and restarts it.

## Signing, and why a rebuild can lose a permission

`perch install` signs the `.app` before installing it. Without `app.sign:` that
signature is ad-hoc, which seals `Info.plist` and your artwork but gives the
bundle **a different identity every time you build it**.

That matters for anything macOS asks you to allow once. A widget that polls a
host on your network needs Local Network access, and macOS remembers that grant
against the app's code signature. An ad-hoc signature has no identity beyond
the exact bytes, so the next `perch install` presents as a different app and
the grant no longer applies. Nothing reports this: a blocked connection still
exits 0, so `.ok` stays true and the widget looks idle rather than broken.

Naming a code signing identity fixes it, because the signature then says who
signed it rather than what the bytes were:

```yaml
app:
  name: worker
  id: dev.example.worker.menubar
  icon: gearshape
  interval: 10s
  sign: perch local signing
```

An identity perch cannot use fails the install rather than falling back to
ad-hoc. The fallback would work, which is exactly what would let you lose a
stable identity — and every permission granted to the app — without noticing.

### Making an identity

Any code signing identity in your keychain works, including an Apple
Development certificate. If you have none, a self-signed one is enough: it is
never checked by anything but your own Mac, and it does not expire on Apple's
schedule.

```
cat > cert.cnf <<'EOF'
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = perch local signing
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
EOF

openssl req -x509 -newkey rsa:2048 -days 7300 -nodes \
  -keyout key.pem -out cert.pem -config cert.cnf
openssl pkcs12 -export -legacy -out bundle.p12 -inkey key.pem -in cert.pem \
  -passout pass:perch -name "perch local signing"
security import bundle.p12 -k ~/Library/Keychains/login.keychain-db \
  -P perch -T /usr/bin/codesign
```

`-legacy` matters: without it OpenSSL 3 writes a PKCS#12 that macOS refuses to
import, reporting a wrong password. Delete the three files afterwards — the
keychain holds what is needed.

`codesign -d -r- ~/Applications/<name>.app` prints what the signature commits
to. With an identity it reads `identifier "<your bundle id>" and certificate
leaf = H"…"` — neither of which a rebuild changes, which is the whole point.

## Completion and errors in your editor

`perch schema` writes the JSON Schema for `menubar.yaml`:

```
perch schema -o menubar.schema.json
```

Point your editor at it with a comment on the first line of the file, and a
misspelled key is underlined as you type rather than at build:

```yaml
# yaml-language-server: $schema=./menubar.schema.json
app: {name: fleet, id: dev.example.fleet, icon: circle, interval: 5s}
menu:
  - {text: Quit, quit: true}
```
