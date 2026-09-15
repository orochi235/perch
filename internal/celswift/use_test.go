package celswift

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/spec"
)

func daemonUse() spec.Use {
	return spec.Use{
		Name:    "daemon",
		Watches: []spec.Watch{{Name: "agent", Kind: spec.WatchLaunchAgent, Label: "dev.example.daemon", Scope: "daemon"}},
		States:  []spec.State{{Name: "stopped", Cond: "!self.agent.loaded"}, {Name: "running"}},
	}
}

func TestSelfReachesAUsesWatchesAndStates(t *testing.T) {
	e := ForUse(daemonUse(), "daemon").Prefixed("results.")
	for src, want := range map[string]string{
		"self.agent.pid": "results.daemon.agent.pid",
		"self.running":   "results.daemon.running",
	} {
		if got, err := e.LowerExpr(src); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", src, got, err, want)
		}
	}
}

// A use's state condition sees its watches and not its states, for the reason
// a file's state condition sees no states: the ordering already decides.
func TestAUsesStateConditionSeesOnlyItsWatches(t *testing.T) {
	e := ForUseStates(daemonUse(), "self")
	if got, err := e.LowerCondition("!self.agent.loaded"); err != nil || got != "!(self.agent.loaded)" {
		t.Errorf("= %q, %v", got, err)
	}
	got, err := e.LowerCondition("self.stopped")
	if err == nil || !strings.Contains(err.Error(), "a state's condition cannot name a state; the order of state: already rules out the earlier ones") {
		t.Errorf("self.stopped = %q, %v; want a refusal saying why", got, err)
	}
}

// Inside a template the use's own name is not bound: self is how it is reached.
func TestATemplateCannotNameItsOwnUse(t *testing.T) {
	_, err := ForUse(daemonUse(), "daemon").LowerExpr("daemon.agent.pid")
	if err == nil || !strings.Contains(err.Error(), `"daemon.agent.pid": unknown name "daemon"; a template sees only self`) {
		t.Errorf("err = %v", err)
	}
}

func TestSelfCarriesTheLaunchAgentHint(t *testing.T) {
	_, err := ForUse(daemonUse(), "daemon").LowerExpr("self.agent.ok")
	if err == nil || !strings.Contains(err.Error(), "A LaunchAgent has two answers") {
		t.Errorf("err = %v", err)
	}
}

func TestHasOnAUsesWatchIsConstant(t *testing.T) {
	if got, err := ForUse(daemonUse(), "daemon").LowerCondition("has(self.agent.pid)"); err != nil || got != "true" {
		t.Errorf("= %q, %v; want true", got, err)
	}
}

// self in a state condition is the struct the state is declared on, so no
// prefix reaches it.
func TestForUseStatesIsNotPrefixed(t *testing.T) {
	e := ForUseStates(daemonUse(), "self").Prefixed("results.")
	if got, err := e.LowerCondition("!self.agent.loaded"); err != nil || got != "!(self.agent.loaded)" {
		t.Errorf("= %q, %v", got, err)
	}
}

func TestTheFileReachesAUseByName(t *testing.T) {
	e := NewEnv(nil).WithUses([]spec.Use{daemonUse()}).Prefixed("results.")
	for src, want := range map[string]string{
		"daemon.running":   "results.daemon.running",
		"daemon.agent.pid": "results.daemon.agent.pid",
	} {
		if got, err := e.LowerExpr(src); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", src, got, err, want)
		}
	}
}

func TestSelfOutsideATemplateSaysWhereItIsBound(t *testing.T) {
	_, err := NewEnv(nil).LowerExpr("self.agent.pid")
	if err == nil || !strings.Contains(err.Error(), "bound only inside a template") {
		t.Errorf("err = %v", err)
	}
}

func TestItAndSelfBothResolveInsideATemplatesEach(t *testing.T) {
	e := ForUse(daemonUse(), "daemon").Prefixed("results.").WithEach(&spec.Type{Kind: spec.TypeString}, "it1")
	if got, err := e.LowerText("it"); err != nil || got != "it1" {
		t.Errorf("it = %q, %v", got, err)
	}
	if got, err := e.LowerExpr("self.agent.pid"); err != nil || got != "results.daemon.agent.pid" {
		t.Errorf("self.agent.pid = %q, %v", got, err)
	}
}
