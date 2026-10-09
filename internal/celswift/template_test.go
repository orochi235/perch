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

func TestLowerURLEncodesOnlyABareQueryHole(t *testing.T) {
	e := shaped(t).WithQuery("query1")
	got, err := e.LowerURL("https://x/?q={{query}}&n={{fleet.data.label}}")
	if err != nil {
		t.Fatalf("LowerURL: %v", err)
	}
	want := `"https://x/?q=\(Act.urlQuery(query1))&n=\(fleet.data.label)"`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestLowerURLWithoutAQueryIsLowerTemplate(t *testing.T) {
	a, _ := shaped(t).LowerURL("https://x/{{fleet.data.label}}")
	b, _ := shaped(t).LowerTemplate("https://x/{{fleet.data.label}}")
	if a != b {
		t.Errorf("LowerURL %q, LowerTemplate %q", a, b)
	}
}

func TestQueryIsAStringInRunArgs(t *testing.T) {
	got, err := runWatch(t).WithQuery("query1").LowerTemplate("{{query}}")
	if err != nil || got != `"\(query1)"` {
		t.Errorf("got %q, %v", got, err)
	}
}
