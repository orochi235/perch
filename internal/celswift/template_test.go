package celswift

import "testing"

func TestLowerTemplateNoHoles(t *testing.T) {
	got, err := runWatch(t).LowerTemplate("Restart server")
	if err != nil {
		t.Fatalf("LowerTemplate: %v", err)
	}
	if got != `"Restart server"` {
		t.Errorf("got %q", got)
	}
}

func TestLowerTemplateInterpolatesHoles(t *testing.T) {
	got, err := shaped(t).LowerTemplate("{{fleet.data.count}} nodes · {{fleet.data.jobs.size()}} running")
	if err != nil {
		t.Fatalf("LowerTemplate: %v", err)
	}
	want := `"\(String(fleet.data.count)) nodes · \(String(fleet.data.jobs.count)) running"`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestLowerTemplateKeepsStringHolesUncast(t *testing.T) {
	got, err := shaped(t).LowerTemplate("job {{fleet.data.label}}")
	if err != nil {
		t.Fatalf("LowerTemplate: %v", err)
	}
	if got != `"job \(fleet.data.label)"` {
		t.Errorf("got %q", got)
	}
}

func TestLowerTemplateEscapesLiteralText(t *testing.T) {
	got, err := runWatch(t).LowerTemplate(`say "hi" \ now`)
	if err != nil {
		t.Fatalf("LowerTemplate: %v", err)
	}
	if got != `"say \"hi\" \\ now"` {
		t.Errorf("got %q", got)
	}
}

func TestLowerTemplateRejectsUnclosedHole(t *testing.T) {
	_, err := runWatch(t).LowerTemplate("{{fleet.ok")
	if err == nil {
		t.Fatal("want an error for an unclosed {{, got nil")
	}
}

func TestLowerTemplateReportsExpressionErrors(t *testing.T) {
	_, err := shaped(t).LowerTemplate("count: {{fleet.data.jbos.size()}}")
	if err == nil {
		t.Fatal("want the hole's expression error to surface, got nil")
	}
	if !contains(err.Error(), "jbos") {
		t.Errorf("error = %q, want it to name the bad field", err)
	}
}

func TestLowerTemplateEmptyHoleIsAnError(t *testing.T) {
	_, err := runWatch(t).LowerTemplate("x{{}}y")
	if err == nil {
		t.Fatal("want an error for an empty hole, got nil")
	}
}
