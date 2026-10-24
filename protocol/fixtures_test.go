package protocol

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/specrig/specrig/guard"
	"gopkg.in/yaml.v3"
)

type fixture struct {
	Specrig string              `yaml:"specrig"`
	Root    string              `yaml:"root"`
	Layout  map[string][]string `yaml:"layout"`
	Areas   []guard.Area        `yaml:"areas"`
	Checks  []struct {
		Mode    string   `yaml:"mode"`
		Path    string   `yaml:"path"`
		Grants  []string `yaml:"grants"`
		Verdict string   `yaml:"verdict"`
		Ways    []string `yaml:"ways"`
	} `yaml:"checks"`
}

// rules are what a stack laid out as the fixture says prints, read back.
func rules(t *testing.T, f fixture) guard.Rules {
	t.Helper()
	body, err := os.ReadFile("modes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Specrig string        `yaml:"specrig"`
		Classes []guard.Class `yaml:"classes"`
		Modes   []guard.Mode  `yaml:"modes"`
	}
	if err := yaml.Unmarshal(body, &table); err != nil {
		t.Fatal(err)
	}
	if table.Specrig != "modes/1" {
		t.Fatalf("modes.yaml is %s", table.Specrig)
	}
	for i, c := range table.Classes {
		table.Classes[i].Patterns = append(c.Patterns, f.Layout[c.Name]...)
	}
	printed, err := json.Marshal(guard.Document{Specrig: "rules/1", Modes: table.Modes, Classes: table.Classes, Areas: f.Areas})
	if err != nil {
		t.Fatal(err)
	}
	d, err := guard.Parse(printed)
	if err != nil {
		t.Fatal(err)
	}
	r, err := guard.New(f.Root, d)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTheProtocolsGuardFixtures(t *testing.T) {
	files, err := filepath.Glob("fixtures/guard/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var f fixture
			if err := yaml.Unmarshal(body, &f); err != nil {
				t.Fatal(err)
			}
			if f.Specrig != "guard-fixtures/1" {
				t.Fatalf("%s is %s", file, f.Specrig)
			}
			r := rules(t, f)
			for _, c := range f.Checks {
				mode, err := r.Mode(c.Mode)
				if err != nil {
					t.Fatal(err)
				}
				abs := c.Path
				if !path.IsAbs(abs) {
					abs = path.Join(f.Root, c.Path)
				}
				v, ways, reason := r.Check(mode, abs, c.Grants)
				if v.String() != c.Verdict || (v == guard.Refused && !slices.Equal(ways, c.Ways)) {
					t.Errorf("%s mode %v, %s: %s %q (%s), want %s %q", c.Mode, c.Grants, c.Path, v, ways, reason, c.Verdict, c.Ways)
				}
			}
		})
	}
}
