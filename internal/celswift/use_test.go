package celswift

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
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
	if got, err := e.LowerCondition("self.stopped"); err == nil {
		t.Errorf("self.stopped lowered to %q; want a refusal", got)
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
