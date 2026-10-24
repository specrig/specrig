package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specrig/specrig/service"
)

// A monorepo with a waffle service and a specrig one, and a guard stand-in that
// answers as the test says and records what it was asked. Unless the test gives
// rules, the stand-in predates them.
type fixture struct {
	root, svc, other string
	h                hook
	rules            string
	calls            []string
}

func newFixture(t *testing.T, answer func(args []string) (string, error)) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{root: service.Resolve(root)}
	for name, marker := range map[string]string{"kostej": "waffle.yaml", "shark": "specrig.yaml"} {
		dir := filepath.Join(root, "services", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, marker), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f.svc = service.Resolve(filepath.Join(root, "services", "kostej"))
	f.other = service.Resolve(filepath.Join(root, "services", "shark"))
	f.h = hook{data: t.TempDir(), run: func(dir string, args ...string) (string, error) {
		if args[0] == "rules" {
			if f.rules == "" || dir != f.other {
				return `Unknown command "rules"`, errors.New("exit status 1")
			}
			return f.rules, nil
		}
		f.calls = append(f.calls, dir+" "+strings.Join(args, " "))
		return answer(args)
	}}
	return f
}

func allowing(args []string) (string, error) { return "brief of " + args[1], nil }

func (f *fixture) prompt(text string) string {
	return f.h.prompt(input{SessionID: "s1", Cwd: f.root, Prompt: text})
}

func (f *fixture) edit(path string) *decision {
	in := input{SessionID: "s1", Cwd: f.root}
	in.ToolInput.FilePath = path
	return f.h.edit(in)
}

func (f *fixture) last() string { return f.calls[len(f.calls)-1] }

func TestPromptSwitchesTheServiceNamedByItsPath(t *testing.T) {
	f := newFixture(t, allowing)
	out := f.prompt("/specrig:spec ./services/kostej PROJ-1 fix the layout")

	st := f.h.load("s1")
	if st.Mode != "spec" || st.modeOf(f.svc) != "spec" || st.modeOf(f.other) != unswitched {
		t.Errorf("state %+v", st)
	}
	if !strings.Contains(out, "switched to spec mode, for services/kostej") || !strings.Contains(out, "brief of spec") {
		t.Errorf("context:\n%s", out)
	}
}

// No layout of a repository is assumed, so a bare name is the task's text even
// when a service directory carries it.
func TestPromptTakesAServiceOnlyByItsPath(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:spec kostej")
	if st := f.h.load("s1"); st.Mode != "" {
		t.Errorf("a bare name switched %+v", st)
	}
}

func TestPromptLeavesOtherTextAlone(t *testing.T) {
	f := newFixture(t, allowing)
	for _, text := range []string{"please fix specrig.yaml", "/waffle:spec ./services/kostej"} {
		if out := f.prompt(text); out != "" || len(f.calls) != 0 {
			t.Errorf("%q produced %q and %d guard calls", text, out, len(f.calls))
		}
	}
}

func TestPromptWithNoServiceSwitchesNothing(t *testing.T) {
	f := newFixture(t, allowing)
	out := f.prompt("/specrig:spec")
	if !strings.Contains(out, "Nothing was switched") || f.h.load("s1").Mode != "" {
		t.Errorf("context %q, state %+v", out, f.h.load("s1"))
	}
}

func TestACommandWithoutAPathActsOnTheServicesNamedBefore(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:spec ./services/kostej ./services/shark")
	f.prompt("/specrig:redgreen")
	st := f.h.load("s1")
	if st.modeOf(f.svc) != "redgreen" || st.modeOf(f.other) != "redgreen" {
		t.Errorf("state %+v", st)
	}
}

func TestNamingFewerServicesResetsTheRest(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:spec ./services/kostej ./services/shark")
	f.prompt("/specrig:redgreen ./services/kostej")
	st := f.h.load("s1")
	if st.modeOf(f.svc) != "redgreen" || st.modeOf(f.other) != unswitched {
		t.Errorf("state %+v", st)
	}
}

func TestEditIsCheckedInTheSessionsMode(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "guard" {
			return "Error: spec mode does not allow specrig.yaml; platform mode does", errors.New("exit status 1")
		}
		return allowing(args)
	})
	f.prompt("/specrig:spec ./services/shark")

	d := f.edit(filepath.Join(f.other, "specrig.yaml"))

	if d == nil || d.HookSpecificOutput == nil || d.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("decision = %+v", d)
	}
	if reason := d.HookSpecificOutput.PermissionDecisionReason; !strings.Contains(reason, "platform mode does") ||
		!strings.Contains(reason, "/specrig:<mode> ./services/shark") {
		t.Errorf("reason = %q", reason)
	}
	if !strings.HasSuffix(f.last(), "guard spec "+filepath.Join(f.other, "specrig.yaml")) {
		t.Errorf("asked the guard %q", f.last())
	}
}

const rules = `{"specrig":"rules/1","modes":[` +
	`{"name":"spec","opens":["cases"],"allows":"the cases","brief":"Write the cases, stop."},` +
	`{"name":"redgreen","opens":["tests","production"],"allows":"the steps and the code","brief":"Make them green, stop."}],` +
	`"classes":[{"name":"cases","patterns":["tests/feature/**/*.feature"]},{"name":"tests","patterns":["tests/"]},{"name":"production"}],` +
	`"areas":[]}`

func TestAServiceWithRulesIsCheckedWithoutAskingItsTool(t *testing.T) {
	f := newFixture(t, allowing)
	f.rules = rules
	out := f.prompt("/specrig:redgreen ./services/shark")

	d := f.edit(filepath.Join(f.other, "tests", "feature", "pay.feature"))

	if !strings.Contains(out, "Mode: redgreen\nMake them green, stop.\nMay be edited: the steps and the code.") {
		t.Errorf("context:\n%s", out)
	}
	if d == nil || d.HookSpecificOutput == nil ||
		d.HookSpecificOutput.PermissionDecisionReason != "redgreen mode does not allow tests/feature/pay.feature; spec mode does — "+
			"stop and ask the human for it. The human switches with /specrig:<mode> ./services/shark." {
		t.Fatalf("decision = %+v", d)
	}
	if d := f.edit(filepath.Join(f.other, "src", "pay.ts")); d != nil || len(f.calls) != 0 {
		t.Errorf("decision %+v, calls %v", d, f.calls)
	}
}

// While a task is on, a service it does not name is closed whole — a grant or a
// platform mode for one service reaches no other.
func TestAServiceOutsideTheTaskIsClosed(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:platform ./services/kostej")
	f.prompt("/specrig:allow gomod")
	calls := len(f.calls)

	d := f.edit(filepath.Join(f.other, "package.json"))

	if d == nil || d.HookSpecificOutput == nil || !strings.Contains(d.HookSpecificOutput.PermissionDecisionReason, "services/shark is not in this task") {
		t.Fatalf("decision = %+v", d)
	}
	if len(f.calls) != calls {
		t.Errorf("a guard was asked about a service outside the task: %v", f.calls[calls:])
	}
}

// An agent staging a commit copies files, a marker among them, into its
// scratchpad; that copy is not a service anyone reviews.
func TestACopyInATemporaryDirectoryIsNotChecked(t *testing.T) {
	f := newFixture(t, allowing)
	scratch := t.TempDir()
	copied := filepath.Join(scratch, "commit", "services", "dungeon")
	if err := os.MkdirAll(copied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copied, "specrig.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f.h.scratch = []string{service.Resolve(scratch)}
	f.prompt("/specrig:redgreen ./services/kostej")
	calls := len(f.calls)

	if d := f.edit(filepath.Join(copied, "Makefile")); d != nil || len(f.calls) != calls {
		t.Errorf("decision %+v, calls %v", d, f.calls[calls:])
	}
}

func TestWithNoModeSwitchedEveryServiceIsInRedgreen(t *testing.T) {
	f := newFixture(t, allowing)
	f.edit(filepath.Join(f.other, "src", "Orders.cs"))
	if want := f.other + " guard redgreen " + filepath.Join(f.other, "src", "Orders.cs"); f.last() != want {
		t.Errorf("asked %q, want %q", f.last(), want)
	}
}

func TestDotDotDotNamesEveryServiceBelow(t *testing.T) {
	for _, arg := range []string{"./...", "./services/...", "services/..."} {
		t.Run(arg, func(t *testing.T) {
			f := newFixture(t, allowing)
			f.prompt("/specrig:platform " + arg)
			st := f.h.load("s1")
			if st.modeOf(f.svc) != "platform" || st.modeOf(f.other) != "platform" {
				t.Errorf("state %+v", st)
			}
		})
	}
}

// Even with a task already on, a selection that finds nothing says so rather than
// falling back to the task's services.
func TestDotDotDotBelowNoServiceSwitchesNothing(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:spec ./services/kostej")
	if err := os.MkdirAll(filepath.Join(f.root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := f.prompt("/specrig:platform ./docs/...")
	if !strings.Contains(out, "names no service") || f.h.load("s1").Mode != "spec" {
		t.Errorf("context %q, state %+v", out, f.h.load("s1"))
	}
}

func TestAGrantReachesTheGuardUntilTheModeChanges(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:redgreen ./services/shark")
	out := f.prompt("/specrig:allow csproj")
	if !strings.Contains(out, "granted csproj") || !strings.Contains(f.last(), "grant redgreen csproj") {
		t.Fatalf("allow: %q, calls %v", out, f.calls)
	}

	f.edit(filepath.Join(f.other, "tests", "Specs.csproj"))
	if !strings.HasSuffix(f.last(), "--grant csproj") {
		t.Errorf("the guard was asked %q", f.last())
	}

	f.prompt("/specrig:spec")
	f.edit(filepath.Join(f.other, "tests", "Specs.csproj"))
	if strings.Contains(f.last(), "--grant") {
		t.Errorf("a grant outlived its mode: %q", f.last())
	}
}

func TestAGrantTheGuardRejectsIsNotKept(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "grant" {
			return "Error: the gomod area is not granted in spec mode", errors.New("exit status 1")
		}
		return allowing(args)
	})
	f.prompt("/specrig:spec ./services/kostej")
	f.prompt("/specrig:allow gomod")
	if granted := f.h.load("s1").Granted; len(granted) != 0 {
		t.Errorf("granted %v", granted)
	}
}

// A path in no service goes to every service the task names, and one that claims
// it decides.
func TestASharedPathIsAskedOfTheTasksServices(t *testing.T) {
	event := filepath.Join("shared", "api", "events", "price.go")
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "guard" && strings.HasSuffix(args[2], event) {
			return "Error: redgreen mode does not allow it; spec mode does", errors.New("exit status 1")
		}
		return allowing(args)
	})
	f.prompt("/specrig:redgreen ./services/kostej")
	d := f.edit(filepath.Join(f.root, event))
	if d == nil || d.HookSpecificOutput == nil {
		t.Fatalf("decision = %+v, calls %v", d, f.calls)
	}
}

func TestASharedPathNoServiceClaimsIsNotChecked(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "guard" {
			return "unclaimed: no area of this service names it", nil
		}
		return allowing(args)
	})
	f.prompt("/specrig:spec ./services/kostej ./services/shark")
	if d := f.edit(filepath.Join(f.root, "docs", "readme.md")); d != nil {
		t.Errorf("decision = %+v", d)
	}
	if len(f.calls) != 4 {
		t.Errorf("expected both services asked, calls %v", f.calls)
	}
}

// A service whose guard predates the modes, or that has none, must stay editable.
func TestAGuardWithNoModesIsWarnedAboutOnceAndNeverBlocks(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) {
		return `Unknown command "guard"`, errors.New("exit status 1")
	})
	first := f.edit(filepath.Join(f.other, "specrig.yaml"))
	second := f.edit(filepath.Join(f.other, "specrig.yaml"))

	if first == nil || first.HookSpecificOutput != nil || !strings.Contains(first.SystemMessage, "no modes") {
		t.Errorf("first = %+v", first)
	}
	if second != nil {
		t.Errorf("second = %+v", second)
	}
}

// redgreen is also where every service is with no task, so its name alone does not
// tell the agent whether the specification was approved.
func TestSwitchingToRedgreenSaysWhetherItIsTheApproval(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:spec ./services/shark")
	if out := f.prompt("/specrig:redgreen"); !strings.Contains(out, "This switch is the approval") {
		t.Errorf("from spec mode:\n%s", out)
	}
	if out := f.prompt("/specrig:redgreen ./services/kostej"); !strings.Contains(out, "This switch is not an approval") {
		t.Errorf("with no task:\n%s", out)
	}
}

func TestSessionStartWithNoModeSaysNoTaskHasStarted(t *testing.T) {
	f := newFixture(t, allowing)
	if out := f.h.sessionStart(input{SessionID: "s1", Cwd: f.root}); !strings.Contains(out, "No mode is switched on: every service is in redgreen") {
		t.Errorf("context:\n%s", out)
	}
}

func TestSessionStartAnnouncesThePluginAndTheMode(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/specrig:redgreen ./services/kostej")
	out := f.h.sessionStart(input{SessionID: "s1", Cwd: f.root})
	if !strings.Contains(out, "specrig plugin active") || !strings.Contains(out, "redgreen, for services/kostej") {
		t.Errorf("context:\n%s", out)
	}
}
