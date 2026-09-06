// Command perch generates a macOS menu bar app from a menubar.yaml.
package main

import (
	"flag"
	"fmt"
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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "build":
		err = runBuild(args)
	case "run":
		err = runRun(args)
	case "install":
		err = runInstall(args)
	case "uninstall":
		err = runUninstall(args)
	case "shape":
		err = runShape(args)
	case "schema":
		err = runSchema(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "perch: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "perch: %v\n", err)
		os.Exit(1)
	}
}

func flags(name string, args []string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	dir := fs.String("C", ".", "project directory holding menubar.yaml")
	return fs, dir
}

// emit loads the project and rewrites menubar/Generated/.
func emit(dir string) (*project.Project, error) {
	p, err := project.Load(dir)
	if err != nil {
		return nil, err
	}
	files, err := swiftappkit.New().Emit(p.Spec)
	if err != nil {
		return nil, err
	}
	if err := p.WriteGenerated(files); err != nil {
		return nil, err
	}
	for _, f := range files {
		fmt.Printf("  %s\n", filepath.Join("menubar", "Generated", f.Name))
	}
	return p, nil
}

func runBuild(args []string) error {
	fs, dir := flags("build", args)
	_ = fs.Parse(args)
	p, err := emit(*dir)
	if err != nil {
		return err
	}
	fmt.Printf("built %s\n", p.Spec.App.Name)
	return nil
}

func runRun(args []string) error {
	fs, dir := flags("run", args)
	_ = fs.Parse(args)
	p, err := emit(*dir)
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
	fmt.Println("compiling…")
	if err := install.BuildBundle(install.BundleOpts{App: app, Sources: sources, Dest: dest}); err != nil {
		return err
	}
	bin := filepath.Join(dest, "Contents", "MacOS", app.Executable)
	fmt.Printf("running %s (ctrl-c to stop)\n", bin)
	c := exec.Command(bin)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func runInstall(args []string) error {
	fs, dir := flags("install", args)
	_ = fs.Parse(args)
	p, err := emit(*dir)
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
	fmt.Println("compiling…")
	if err := install.BuildBundle(install.BundleOpts{App: app, Sources: sources, Dest: bundle}); err != nil {
		return err
	}
	fmt.Printf("  %s\n", bundle)

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	bin := filepath.Join(bundle, "Contents", "MacOS", app.Executable)
	if err := os.WriteFile(plistPath, []byte(install.AgentPlist(app.ID, bin, install.InstallPATH())), 0o644); err != nil {
		return err
	}
	fmt.Printf("  %s\n", plistPath)

	fmt.Println("loading…")
	if err := install.Reload(install.NewCLI(), app.ID, plistPath, install.DefaultWait()); err != nil {
		return err
	}
	fmt.Printf("%s is running\n", app.Name)
	return nil
}

func runUninstall(args []string) error {
	fs, dir := flags("uninstall", args)
	_ = fs.Parse(args)
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
	if err := install.NewCLI().Bootout(p.Spec.App.ID); err != nil {
		fmt.Printf("  %s was not loaded\n", p.Spec.App.ID)
	}
	for _, path := range []string{plistPath, bundle} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		fmt.Printf("  removed %s\n", path)
	}
	fmt.Printf("%s is uninstalled\n", p.Spec.App.Name)
	return nil
}

func runShape(args []string) error {
	fs := flag.NewFlagSet("shape", flag.ExitOnError)
	from := fs.String("from", "", "command to run once, as argv (no shell)")
	_ = fs.Parse(args)
	if *from == "" {
		return fmt.Errorf("shape needs --from '<command>'")
	}
	argv := strings.Fields(*from)
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	if err != nil {
		return fmt.Errorf("running %s: %w", *from, err)
	}
	body, err := shape.Infer(out)
	if err != nil {
		return err
	}
	fmt.Print("shape:\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		fmt.Printf("  %s\n", line)
	}
	return nil
}

func runSchema(args []string) error {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	out := fs.String("o", "", "write to a file instead of stdout")
	_ = fs.Parse(args)
	body := schema.JSON()
	if *out == "" {
		fmt.Print(body)
		return nil
	}
	return os.WriteFile(*out, []byte(body), 0o644)
}
