package install

import (
	"errors"
	"strings"
	"testing"
)

// fakeLaunchctl models launchd's actual timing: bootout returns before launchd
// has let go of the label, so print keeps succeeding for a few more calls.
type fakeLaunchctl struct {
	loaded       bool
	releaseAfter int // print calls still reporting the job after bootout
	calls        []string
}

func (f *fakeLaunchctl) Print(label string) error {
	f.calls = append(f.calls, "print")
	if !f.loaded {
		return errors.New("could not find service")
	}
	if f.releaseAfter > 0 {
		f.releaseAfter--
		return nil
	}
	return errors.New("could not find service")
}

func (f *fakeLaunchctl) Bootout(label string) error {
	f.calls = append(f.calls, "bootout")
	if !f.loaded {
		return errors.New("no such process")
	}
	return nil
}

func (f *fakeLaunchctl) Bootstrap(plist string) error {
	f.calls = append(f.calls, "bootstrap")
	if f.releaseAfter > 0 {
		return errors.New("Input/output error")
	}
	f.loaded = true
	return nil
}

func TestReloadWaitsForLaunchdToLetGoBeforeBootstrapping(t *testing.T) {
	f := &fakeLaunchctl{loaded: true, releaseAfter: 3}
	var slept int
	if err := Reload(f, "dev.x", "/tmp/x.plist", Wait{Tries: 10, Sleep: func() { slept++ }}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	got := strings.Join(f.calls, " ")
	if !strings.HasPrefix(got, "bootout print") {
		t.Errorf("calls = %q, want bootout before the first print", got)
	}
	if !strings.HasSuffix(got, "bootstrap") {
		t.Errorf("calls = %q, want bootstrap last", got)
	}
	if strings.Count(got, "bootstrap") != 1 {
		t.Errorf("calls = %q, want exactly one bootstrap (never into the gap)", got)
	}
	if slept == 0 {
		t.Error("Reload did not wait between print calls")
	}
}

func TestReloadBootstrapsImmediatelyWhenNothingIsLoaded(t *testing.T) {
	f := &fakeLaunchctl{loaded: false}
	if err := Reload(f, "dev.x", "/tmp/x.plist", Wait{Tries: 10, Sleep: func() {}}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !f.loaded {
		t.Error("job was not bootstrapped")
	}
}

func TestReloadGivesUpRatherThanBootstrappingIntoTheGap(t *testing.T) {
	f := &fakeLaunchctl{loaded: true, releaseAfter: 100}
	err := Reload(f, "dev.x", "/tmp/x.plist", Wait{Tries: 3, Sleep: func() {}})
	if err == nil {
		t.Fatal("want an error when launchd never lets go, got nil")
	}
	if !strings.Contains(err.Error(), "dev.x") {
		t.Errorf("error = %q, want it to name the label", err)
	}
	if strings.Contains(strings.Join(f.calls, " "), "bootstrap") {
		t.Error("Reload bootstrapped anyway; that is the Input/output error this loop exists to avoid")
	}
}
