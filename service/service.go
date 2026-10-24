// Package service finds the service of a path and answers guard/1 for it: from
// the rules its stack's tool prints, rules/1, or by asking a tool older than
// them.
package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A directory holding one of these is a service, and its tool answers for it.
var markers = []string{"specrig.yaml", "waffle.yaml"}

func Marked(dir string) bool {
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// Of walks up from dir, which may not exist yet, to the nearest directory holding
// a marker.
func Of(dir string) string {
	for d := filepath.Clean(dir); ; {
		if Marked(d) {
			return Resolve(d)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// Below takes `dir/...`, which names every service below dir as `go test ./...`
// names packages: still a path, so no layout is assumed. The second result says
// whether arg had that form at all.
func Below(cwd, arg string) ([]string, bool) {
	base, ok := strings.CutSuffix(filepath.ToSlash(arg), "...")
	if !ok || (base != "" && !strings.HasSuffix(base, "/")) {
		return nil, false
	}
	root := filepath.FromSlash(base)
	if !filepath.IsAbs(root) {
		root = filepath.Join(cwd, root)
	}
	var found []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil || !d.IsDir():
			return nil
		case p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "node_modules"):
			return filepath.SkipDir
		}
		if Marked(p) {
			found = append(found, Resolve(p))
			return filepath.SkipDir
		}
		return nil
	})
	return found, true
}

// Named takes an argument that names a service only by its path: no layout of a
// repository is assumed. Anything else is the task's text.
func Named(cwd, arg string) string {
	path := arg
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, arg)
	}
	if Marked(path) {
		return Resolve(path)
	}
	return ""
}

func Resolve(dir string) string {
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		return r
	}
	return dir
}
