package install

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Launchctl is the launchd surface install needs. It is an interface so the
// wait loop can be tested without loading anything.
type Launchctl interface {
	Print(label string) error // nil while launchd still knows the label
	Bootout(label string) error
	Bootstrap(plist string) error
}

// Wait tunes the poll between bootout and bootstrap.
type Wait struct {
	Tries int
	Sleep func()
}

// DefaultWait polls for about two seconds.
func DefaultWait() Wait {
	return Wait{Tries: 40, Sleep: func() { time.Sleep(50 * time.Millisecond) }}
}

// Reload replaces a running agent. bootout returns before launchd has let go of
// the label, and bootstrapping into that gap fails with Input/output error — so
// this polls until the label is genuinely gone, and reports rather than
// bootstrapping blind if it never is.
func Reload(lc Launchctl, label, plist string, w Wait) error {
	_ = lc.Bootout(label) // not loaded is not a failure

	for i := 0; i < w.Tries; i++ {
		if err := lc.Print(label); err != nil {
			return lc.Bootstrap(plist)
		}
		w.Sleep()
	}
	return fmt.Errorf("launchd still holds %s after %d checks; not bootstrapping into the gap", label, w.Tries)
}

// CLI runs the real launchctl against the caller's GUI domain.
type CLI struct{ UID int }

func NewCLI() *CLI { return &CLI{UID: os.Getuid()} }

func (c *CLI) domain() string { return "gui/" + strconv.Itoa(c.UID) }

func (c *CLI) Print(label string) error {
	return exec.Command("launchctl", "print", c.domain()+"/"+label).Run()
}

func (c *CLI) Bootout(label string) error {
	return exec.Command("launchctl", "bootout", c.domain()+"/"+label).Run()
}

func (c *CLI) Bootstrap(plist string) error {
	out, err := exec.Command("launchctl", "bootstrap", c.domain(), plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, out)
	}
	return nil
}
