package service

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestTheAnswerIsFoundBehindWhatALauncherPrints(t *testing.T) {
	if v, text := classify("Building...\nError: spec mode does not allow src/Program.cs", errors.New("exit status 1")); v != Refused ||
		text != "spec mode does not allow src/Program.cs" {
		t.Errorf("refusal: %v %q", v, text)
	}
	if v, text := classify("go: downloading x\nunclaimed: no area names it", nil); v != Unclaimed || text != "unclaimed: no area names it" {
		t.Errorf("unclaimed: %v %q", v, text)
	}
}

func TestTheMarkerNamesTheGuard(t *testing.T) {
	cases := map[string][]string{
		"guard: [dotnet, specrig]\n":                           {"dotnet", "specrig"},
		"flows:\n  clients: {}\nguard: [npx, --no, specrig]\n": {"npx", "--no", "specrig"},
		"guard:\n  - dotnet\n  - specrig # the tool\n":         {"dotnet", "specrig"},
		"guard:\n- php\n- vendor/bin/specrig\nflows:\n":        {"php", "vendor/bin/specrig"},
		"guard: \"dotnet specrig\"\n":                          {"dotnet", "specrig"},
		"flows:\n  guard: [nope]\n":                            nil,
	}
	for body, want := range cases {
		if got := guardIn(body); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", body, got, want)
		}
	}
}

func TestEachMarkerIsAskedInItsOwnWords(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "specrig.yaml"), []byte("guard: [dotnet, specrig]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if argv, err := toolOf(dir, "rules"); err != nil || !slices.Equal(argv, []string{"dotnet", "specrig", "rules"}) {
		t.Errorf("specrig.yaml: %q, %v", argv, err)
	}

	waffle := t.TempDir()
	if err := os.WriteFile(filepath.Join(waffle, "waffle.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAFFLE_BIN", "")
	if argv, err := toolOf(waffle, "mode"); err != nil || !slices.Equal(argv, []string{"go", "tool", "waffle", "agents:mode"}) {
		t.Errorf("waffle.yaml: %q, %v", argv, err)
	}

	bare := t.TempDir()
	if err := os.WriteFile(filepath.Join(bare, "specrig.yaml"), []byte("flows: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Tool(bare, "rules")
	if v, _ := classify(out, err); v != NoModes {
		t.Errorf("a specrig.yaml naming no guard: %q, %v", out, err)
	}
}
