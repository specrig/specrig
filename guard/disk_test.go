package guard

import (
	"os"
	"path/filepath"
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
