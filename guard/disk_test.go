package guard

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestASymlinkDoesNotOpenWhatItsTargetsTableCloses(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"tests/feature", "src"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	feature := filepath.Join(root, "tests", "feature", "pay.feature")
	if err := os.WriteFile(feature, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "src", "pay.feature")
	if err := os.Symlink(feature, link); err != nil {
		t.Fatal(err)
	}
	r, err := NewOnDisk(root, document(map[string][]string{"cases": {"tests/feature/**/*.feature"}, "tests": {"tests/"}}))
	if err != nil {
		t.Fatal(err)
	}

	if v, ways, _, err := r.CheckPath(mode(t, r, "redgreen"), link, nil); err != nil || v != Refused || len(ways) != 1 || ways[0] != "spec mode" {
		t.Errorf("verdict %v, ways %q, err %v", v, ways, err)
	}
}

func TestASymlinkDoesNotTakeAPathOutOfTheServiceReachedThroughALinkToIt(t *testing.T) {
	top := t.TempDir()
	for _, dir := range []string{"real/src", "elsewhere"} {
		if err := os.MkdirAll(filepath.Join(top, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(top, "real"), filepath.Join(top, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(top, "elsewhere"), filepath.Join(top, "real", "src", "out")); err != nil {
		t.Fatal(err)
	}
	r, err := NewOnDisk(filepath.Join(top, "alias"), document(nil))
	if err != nil {
		t.Fatal(err)
	}

	v, ways, _, err := r.CheckPath(mode(t, r, "spec"), filepath.Join(top, "alias", "src", "out", "main.ts"), nil)
	if err != nil || v != Refused || !slices.Equal(ways, []string{"redgreen mode", "platform mode"}) {
		t.Errorf("verdict %v, ways %q, err %v", v, ways, err)
	}
}

func TestAnAreaClaimsWhereItsPathLeadsAsWellAsWhereItIsWritten(t *testing.T) {
	top := t.TempDir()
	for _, dir := range []string{"app", "repo/shared/events"} {
		if err := os.MkdirAll(filepath.Join(top, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(top, "repo", "shared"), filepath.Join(top, "shared")); err != nil {
		t.Fatal(err)
	}
	r, err := NewOnDisk(filepath.Join(top, "app"), document(nil, Area{Name: "events", Paths: []string{"../shared/events/"}, Modes: []string{"spec"}}))
	if err != nil {
		t.Fatal(err)
	}

	for _, written := range []string{filepath.Join(top, "shared", "events", "paid.json"), filepath.Join(top, "repo", "shared", "events", "paid.json")} {
		if v, _, _, err := r.CheckPath(mode(t, r, "spec"), written, nil); err != nil || v != Allowed {
			t.Errorf("%s in spec: %v, %v", written, v, err)
		}
		if _, ways, _, err := r.CheckPath(mode(t, r, "redgreen"), written, nil); err != nil || !slices.Equal(ways, []string{"spec mode"}) {
			t.Errorf("%s in redgreen: ways %q, %v", written, ways, err)
		}
	}
}

func TestAPathIsSpelledAsItsDirectoryListsIt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Makefile"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := NewOnDisk(root, document(map[string][]string{"platform": {"Makefile"}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "MAKEFILE")); err != nil {
		t.Skip("the filesystem tells the spellings apart")
	}

	if v, ways, _, err := r.CheckPath(mode(t, r, "redgreen"), filepath.Join(root, "MAKEFILE"), nil); err != nil || v != Refused || len(ways) != 1 || ways[0] != "platform mode" {
		t.Errorf("verdict %v, ways %q, err %v", v, ways, err)
	}
}
