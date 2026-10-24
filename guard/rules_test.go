package guard

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func document(layout map[string][]string, areas ...Area) Document {
	classes := []Class{{Name: "humans", Patterns: []string{"CONTRIBUTING.md"}}, {Name: "generated", Patterns: []string{"specrig.md"}},
		{Name: "platform"}, {Name: "cases"}, {Name: "tests"}, {Name: "production"}}
	for i := range classes {
		classes[i].Patterns = append(classes[i].Patterns, layout[classes[i].Name]...)
	}
	hidden := false
	return Document{Specrig: "rules/1", Classes: classes, Areas: areas, Modes: []Mode{
		{Name: "spec", Opens: []string{"cases"}, Allows: "`tests/feature/**/*.feature`", Brief: "Write the cases."},
		{Name: "redgreen", Opens: []string{"tests", "production"}},
		{Name: "platform", Opens: []string{"platform", "production"}},
		{Name: "auto", Opens: []string{"cases", "tests", "production"}, Includes: []string{"spec", "redgreen"}, Offered: &hidden},
	}}
}

func mode(t *testing.T, r Rules, name string) Mode {
	t.Helper()
	m, err := r.Mode(name)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRulesOfAnotherVersionAreNotRead(t *testing.T) {
	if _, err := Parse([]byte(`{"specrig":"rules/2","modes":[]}`)); err == nil || !strings.Contains(err.Error(), "rules/1") {
		t.Errorf("err = %v", err)
	}
}

func TestAnAreaNamingAModeTheRulesLackIsNamed(t *testing.T) {
	_, err := New("/svc", document(nil, Area{Name: "deps", Paths: []string{"go.mod"}, Grant: []string{"tdd"}}))
	if err == nil || !strings.Contains(err.Error(), "agents.areas.deps") || !strings.Contains(err.Error(), `"tdd"`) {
		t.Errorf("err = %v", err)
	}
}

// A tool keeps the rules it read as JSON, and an area closed in every mode must
// not come back as one the class table decides.
func TestAnAreaClosedInEveryModeStaysClosedThroughJSON(t *testing.T) {
	d := document(nil, Area{Name: "pb", Paths: []string{"api/pb/"}, Modes: []string{}}, Area{Name: "steps", Paths: []string{"tests/steps/"}})
	body, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	read, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if read.Areas[0].Modes == nil || read.Areas[1].Modes != nil {
		t.Errorf("areas %+v", read.Areas)
	}
}

func TestAGrantIsTakenOnlyInTheModesTheAreaNames(t *testing.T) {
	r, err := New("/svc", document(nil,
		Area{Name: "deps", Paths: []string{"go.mod"}, Modes: []string{"platform"}, Grant: []string{"redgreen"}},
		Area{Name: "steps", Paths: []string{"tests/steps/"}, Grant: []string{"platform"}}))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := r.Grant(mode(t, r, "auto"), "deps"); err != nil || out != "Grant: deps, on top of auto mode, until the mode changes.\nMay also be edited: go.mod." {
		t.Errorf("deps in auto: %q, %v", out, err)
	}
	if _, err := r.Grant(mode(t, r, "platform"), "deps"); err == nil || !strings.Contains(err.Error(), "open in platform mode already") {
		t.Errorf("deps in platform: %v", err)
	}
	if _, err := r.Grant(mode(t, r, "spec"), "steps"); err == nil || !strings.Contains(err.Error(), "(it is in: platform)") {
		t.Errorf("steps in spec: %v", err)
	}
	if got, want := r.Grants(mode(t, r, "platform")), "Open in platform mode on their own:\n  deps — go.mod\nGrants in platform mode:\n  steps — tests/steps/"; got != want {
		t.Errorf("listing\n%s\nwant\n%s", got, want)
	}
}

func TestABriefIsWhatTheModeAllows(t *testing.T) {
	r, err := New("/svc", document(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := Brief(mode(t, r, "spec")); got != "Mode: spec\nWrite the cases.\nMay be edited: `tests/feature/**/*.feature`." {
		t.Errorf("brief %q", got)
	}
}

func TestAModeNotOfferedIsNeverAWay(t *testing.T) {
	r, err := New("/svc", document(map[string][]string{"cases": {"tests/feature/**/*.feature"}},
		Area{Name: "flows", Paths: []string{"tests/flows.yaml"}, Modes: []string{"auto", "platform"}}))
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]string{"tests/feature/pay.feature": {"spec mode"}, "tests/flows.yaml": {"platform mode"}} {
		if v, ways, _ := r.Check(mode(t, r, "redgreen"), "/svc/"+path, nil); v != Refused || !slices.Equal(ways, want) {
			t.Errorf("%s: verdict %v, ways %q", path, v, ways)
		}
	}
}
