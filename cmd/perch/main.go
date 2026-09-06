// Command perch generates a macOS menu bar app from a menubar.yaml.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/orochi235/perch/internal/backend/swiftappkit"
	"github.com/orochi235/perch/internal/install"
	"github.com/orochi235/perch/internal/project"
	"github.com/orochi235/perch/internal/schema"
	"github.com/orochi235/perch/internal/shape"
)

const usage = `perch generates a macOS menu bar app from a menubar.yaml.

  perch build         emit Swift into menubar/Generated/
  perch run           build, compile, run in the foreground
  perch install       build, compile, bundle, write the plist, bootstrap
  perch uninstall     bootout and remove
  perch shape         run a command once and write its shape declaration
  perch schema        write the JSON Schema for menubar.yaml

Flags come after the command; -C sets the project directory.
`

// env is everything a command reaches the outside world through, so a test can
// drive the whole of install without compiling Swift or loading a LaunchAgent.
type env struct {
	out, err io.Writer
	compile  install.Compiler       // nil compiles with swiftc
	launchd  install.Launchctl      // nil drives the real launchctl
	wait     install.Wait           // zero polls for about two seconds
	start    func(bin string) error // nil runs the binary in the foreground
}

func (e *env) launchctl() install.Launchctl {
	if e.launchd != nil {
		return e.launchd
	}
	return install.NewCLI()
}

func (e *env) waiter() install.Wait {
	if e.wait.Tries > 0 {
		return e.wait
	}
	return install.DefaultWait()
}

func (e *env) run(bin string) error {
	if e.start != nil {
		return e.start(bin)
	}
	c := exec.Command(bin)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func main() {
	os.Exit(run(os.Args[1:], &env{out: os.Stdout, err: os.Stderr}))
}

// run is main without the process: it returns the exit status rather than
// taking it, so every command is reachable from a test.
func run(args []string, e *env) int {
	if len(args) == 0 {
		fmt.Fprint(e.err, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]

	var err error
	switch cmd {
	case "build":
		err = runBuild(rest, e)
	case "run":
		err = runRun(rest, e)
	case "install":
		err = runInstall(rest, e)
	case "uninstall":
		err = runUninstall(rest, e)
	case "shape":
		err = runShape(rest, e)
	case "schema":
		err = runSchema(rest, e)
	case "-h", "--help", "help":
		fmt.Fprint(e.out, usage)
		return 0
	default:
		fmt.Fprintf(e.err, "perch: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	if err != nil {
		// The flag package has already printed the complaint and the flags the
		// command takes; saying it again just buries the usage.
		var bad badFlag
		if errors.As(err, &bad) {
			return 2
		}
		fmt.Fprintf(e.err, "perch: %v\n", err)
		return 1
	}
	return 0
}

// badFlag is a flag the command does not take, which is a usage failure rather
// than a failure of the work.
type badFlag struct{ error }

// parse reports a flag error as a usage failure.
func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return badFlag{err}
	}
	return nil
}

// newFlags reports a bad flag rather than exiting, so a test sees the status
// run would have returned.
func newFlags(name string, e *env) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(e.err)
	return fs
}

func flags(name string, e *env) (*flag.FlagSet, *string) {
	fs := newFlags(name, e)
	dir := fs.String("C", ".", "project directory holding menubar.yaml")
	return fs, dir
}

// emit loads the project and rewrites menubar/Generated/.
func emit(dir string, e *env) (*project.Project, error) {
	p, err := project.Load(dir)
	if err != nil {
		return nil, err
	}
	files, err := swiftappkit.New().Emit(p.Spec)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.SpecPath(), err)
	}
	if err := p.WriteGenerated(files); err != nil {
		return nil, err
	}
	for _, f := range files {
		fmt.Fprintf(e.out, "  %s\n", filepath.Join("menubar", "Generated", f.Name))
	}
	return p, nil
}

func runBuild(args []string, e *env) error {
	fs, dir := flags("build", e)
	if err := parse(fs, args); err != nil {
		return err
	}
	p, err := emit(*dir, e)
	if err != nil {
		return err
	}
	fmt.Fprintf(e.out, "built %s\n", p.Spec.App.Name)
	return nil
}

func runRun(args []string, e *env) error {
	fs, dir := flags("run", e)
	if err := parse(fs, args); err != nil {
		return err
	}
	p, err := emit(*dir, e)
	if err != nil {
		return err
	}
	sources, err := p.SwiftSources()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "perch-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	app := install.App{Name: p.Spec.App.Name, ID: p.Spec.App.ID, Executable: p.Spec.App.Name}
	dest := filepath.Join(tmp, p.Spec.App.Name+".app")
	fmt.Fprintln(e.out, "compiling…")
	if err := install.BuildBundle(install.BundleOpts{App: app, Sources: sources, Dest: dest, Compile: e.compile}); err != nil {
		return err
	}
	bin := filepath.Join(dest, "Contents", "MacOS", app.Executable)
	fmt.Fprintf(e.out, "running %s (ctrl-c to stop)\n", bin)
	return e.run(bin)
}

func runInstall(args []string, e *env) error {
	fs, dir := flags("install", e)
	if err := parse(fs, args); err != nil {
		return err
	}
	p, err := emit(*dir, e)
	if err != nil {
		return err
	}
	sources, err := p.SwiftSources()
	if err != nil {
		return err
	}
	bundle, err := p.BundlePath()
	if err != nil {
		return err
	}
	plistPath, err := p.PlistPath()
	if err != nil {
		return err
	}

	app := install.App{Name: p.Spec.App.Name, ID: p.Spec.App.ID, Executable: p.Spec.App.Name}
	fmt.Fprintln(e.out, "compiling…")
	if err := install.BuildBundle(install.BundleOpts{App: app, Sources: sources, Dest: bundle, Compile: e.compile}); err != nil {
		return err
	}
	fmt.Fprintf(e.out, "  %s\n", bundle)

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	bin := filepath.Join(bundle, "Contents", "MacOS", app.Executable)
	if err := os.WriteFile(plistPath, []byte(install.AgentPlist(app.ID, bin, install.InstallPATH())), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(e.out, "  %s\n", plistPath)

	fmt.Fprintln(e.out, "loading…")
	if err := install.Reload(e.launchctl(), app.ID, plistPath, e.waiter()); err != nil {
		return err
	}
	fmt.Fprintf(e.out, "%s is running\n", app.Name)
	if len(p.Spec.Watches) > 0 {
		fmt.Fprintf(e.out, "\nIf a watch reaches the network or a .local host, grant %s access under\n"+
			"System Settings -> Privacy & Security -> Local Network. Until you do, those\n"+
			"connections fail silently: the command still exits 0, so .ok stays true and the\n"+
			"widget looks idle rather than broken.\n", app.Name)
	}
	return nil
}

func runUninstall(args []string, e *env) error {
	fs, dir := flags("uninstall", e)
	if err := parse(fs, args); err != nil {
		return err
	}
	p, err := project.Load(*dir)
	if err != nil {
		return err
	}
	bundle, err := p.BundlePath()
	if err != nil {
		return err
	}
	plistPath, err := p.PlistPath()
	if err != nil {
		return err
	}
	if err := e.launchctl().Bootout(p.Spec.App.ID); err != nil {
		fmt.Fprintf(e.out, "  %s was not loaded\n", p.Spec.App.ID)
	}
	for _, path := range []string{plistPath, bundle} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		fmt.Fprintf(e.out, "  removed %s\n", path)
	}
	fmt.Fprintf(e.out, "%s is uninstalled\n", p.Spec.App.Name)
	return nil
}

// repeated collects a flag given more than once.
type repeated []string

func (r *repeated) String() string     { return strings.Join(*r, ", ") }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }

func runShape(args []string, e *env) error {
	fs := newFlags("shape", e)
	var from, samples repeated
	fs.Var(&from, "from", "command to run once, as argv (no shell); repeatable")
	fs.Var(&samples, "sample", "JSON file to read instead of running a command; repeatable")
	if err := parse(fs, args); err != nil {
		return err
	}
	if len(from) == 0 && len(samples) == 0 {
		return fmt.Errorf("shape needs --from '<command>' or --sample <file>")
	}

	var docs [][]byte
	for _, cmd := range from {
		argv := strings.Fields(cmd)
		if len(argv) == 0 {
			return fmt.Errorf("--from: empty; it takes the argv of a command")
		}
		out, err := runOnce(argv)
		if err != nil {
			return fmt.Errorf("running %s: %w", cmd, err)
		}
		docs = append(docs, out)
	}
	for _, path := range samples {
		out, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		docs = append(docs, out)
	}

	body, err := shape.InferAll(docs)
	if err != nil {
		return err
	}
	fmt.Fprint(e.out, "shape:\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		fmt.Fprintf(e.out, "  %s\n", line)
	}
	return nil
}

// runOnce captures a sample. stderr goes into the error because a command that
// prints its complaint there is the usual reason shape has nothing to read.
func runOnce(argv []string) ([]byte, error) {
	var stderr bytes.Buffer
	c := exec.Command(argv[0], argv[1:]...)
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}

func runSchema(args []string, e *env) error {
	fs := newFlags("schema", e)
	out := fs.String("o", "", "write to a file instead of stdout")
	if err := parse(fs, args); err != nil {
		return err
	}
	body := schema.JSON()
	if *out == "" {
		fmt.Fprint(e.out, body)
		return nil
	}
	return os.WriteFile(*out, []byte(body), 0o644)
}
