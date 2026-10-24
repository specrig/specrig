package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specrig/specrig/guard"
)

func printed(t *testing.T) string {
	t.Helper()
	hidden := false
	body, err := json.Marshal(guard.Document{
		Specrig: "rules/1",
		Classes: []guard.Class{{Name: "humans", Patterns: []string{"CONTRIBUTING.md"}}, {Name: "generated", Patterns: []string{"specrig.md"}},
			{Name: "platform", Patterns: []string{"specrig.yaml"}}, {Name: "cases", Patterns: []string{"tests/feature/**/*.feature"}},
			{Name: "tests", Patterns: []string{"tests/"}}, {Name: "production"}},
		Modes: []guard.Mode{
			{Name: "none", Opens: []string{"tests", "production"}, Offered: &hidden},
			{Name: "spec", Opens: []string{"cases"}, Allows: "`tests/feature/**/*.feature`", Brief: "Write the cases."},
			{Name: "redgreen", Opens: []string{"tests", "production"}},
			{Name: "platform", Opens: []string{"platform", "production"}},
		},
		Areas: []guard.Area{{Name: "deps", Paths: []string{"package.json"}, Modes: []string{"platform"}, Grant: []string{"redgreen"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// A service whose tool answers as the test says, and the commands it was asked.
type fixture struct {
	svc   string
	g     Guard
	calls []string
}

func newFixture(t *testing.T, answer func(args []string) (string, error)) *fixture {
	t.Helper()
	f := &fixture{svc: Resolve(t.TempDir())}
	if err := os.WriteFile(filepath.Join(f.svc, "specrig.yaml"), []byte("guard: [npx, --no, specrig]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.g = Guard{Cache: t.TempDir(), Scope: "s1", Run: func(dir string, args ...string) (string, error) {
		f.calls = append(f.calls, strings.Join(args, " "))
		return answer(args)
	}}
	return f
}

func (f *fixture) asked(command string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, command+" ") || c == command {
			n++
		}
	}
	return n
}

func (f *fixture) path(rel string) string { return filepath.Join(f.svc, rel) }

func TestTheRulesAnswerEveryQuestion(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) { return printed(t), nil })

	if v, text := f.g.Check(f.svc, "redgreen", f.path("tests/feature/pay.feature"), nil); v != Refused ||
		text != "redgreen mode does not allow tests/feature/pay.feature; spec mode does — stop and ask the human for it" {
		t.Errorf("a case in redgreen: %v %q", v, text)
	}
	if v, _ := f.g.Check(f.svc, "redgreen", f.path("package.json"), []string{"deps"}); v != OK {
		t.Errorf("a granted area: %v", v)
	}
	if v, text := f.g.Check(f.svc, "spec", filepath.Join(filepath.Dir(f.svc), "docs", "x.md"), nil); v != Unclaimed || !strings.HasPrefix(text, "unclaimed: ") {
		t.Errorf("outside: %v %q", v, text)
	}
	if v, text := f.g.Mode(f.svc, "spec"); v != OK || text != "Mode: spec\nWrite the cases.\nMay be edited: `tests/feature/**/*.feature`." {
		t.Errorf("mode: %v %q", v, text)
	}
	if v, text := f.g.Mode(f.svc, "tdd"); v != Refused || !strings.Contains(text, `unknown mode "tdd"`) {
		t.Errorf("an unknown mode: %v %q", v, text)
	}
	if v, text := f.g.Grant(f.svc, "spec", "deps"); v != Refused || !strings.Contains(text, "not granted in spec mode") {
		t.Errorf("grant: %v %q", v, text)
	}
	if f.asked("rules") != 1 || len(f.calls) != 1 {
		t.Errorf("calls %q", f.calls)
	}
}

func TestRulesAreReadAgainInAnotherScope(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) { return printed(t), nil })
	f.g.Mode(f.svc, "spec")
	f.g.Scope = "s2"
	f.g.Mode(f.svc, "spec")
	if f.asked("rules") != 2 {
		t.Errorf("calls %q", f.calls)
	}
}

// Platform mode edits the marker — an area, the tool it names — and the next
// edit is checked against what it says now.
func TestRulesAreReadAgainWhenTheMarkerChanges(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) { return printed(t), nil })
	f.g.Mode(f.svc, "spec")
	if err := os.WriteFile(f.path("specrig.yaml"), []byte("guard: [npx, --no, specrig]\nagents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.g.Mode(f.svc, "spec")
	if f.asked("rules") != 2 {
		t.Errorf("calls %q", f.calls)
	}
}

func TestAToolOlderThanRulesIsAskedTheCommands(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "rules" {
			return `Unknown command "rules"`, errors.New("exit status 1")
		}
		return "", nil
	})
	f.g.Check(f.svc, "redgreen", f.path("src/a.ts"), []string{"deps"})
	f.g.Grant(f.svc, "redgreen", "")
	if f.asked("rules") != 1 || f.calls[1] != "guard redgreen "+f.path("src/a.ts")+" --grant deps" || f.calls[2] != "grant redgreen" {
		t.Errorf("calls %q", f.calls)
	}
}

func TestRulesAreFoundBehindWhatALauncherPrints(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) {
		return "Tool 'specrig' (version '0.3.0') was restored.\n" + printed(t), nil
	})
	if v, text := f.g.Mode(f.svc, "spec"); v != OK {
		t.Errorf("mode: %v %q", v, text)
	}
}

func TestRulesThatDoNotReadAreAGuardThatDidNotRun(t *testing.T) {
	for name, out := range map[string]string{
		"not JSON":           `{"specrig": "rules/1",`,
		"another version":    `{"specrig":"rules/2"}`,
		"an unknown mode":    strings.Replace(printed(t), `"grant":["redgreen"]`, `"grant":["tdd"]`, 1),
		"nothing like rules": "ok",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, func([]string) (string, error) { return out, nil })
			if v, text := f.g.Check(f.svc, "spec", f.path("src/a.ts"), nil); v != Broken || !strings.HasPrefix(text, "rules: ") {
				t.Errorf("%v %q", v, text)
			}
		})
	}
}

// The tool refuses what the marker says — an area with no paths — and every edit
// is refused with its reason until the marker is fixed.
func TestAToolRefusingItsRulesRefusesEveryEdit(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) {
		return "Error: specrig.yaml: agents.areas.deps names no paths", errors.New("exit status 1")
	})
	for range 2 {
		if v, text := f.g.Check(f.svc, "redgreen", f.path("src/a.ts"), nil); v != Refused || text != "specrig.yaml: agents.areas.deps names no paths" {
			t.Errorf("%v %q", v, text)
		}
	}
	if f.asked("rules") != 2 {
		t.Errorf("calls %q", f.calls)
	}
}
