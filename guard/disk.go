package guard

import (
	"os"
	"path/filepath"
	"strings"
)

// NewOnDisk is New for a service on disk: root is resolved, and an area's path
// claims both where it is written and where it leads.
func NewOnDisk(root string, d Document) (Rules, error) {
	resolved, err := ResolveExisting(root)
	if err != nil {
		return Rules{}, err
	}
	r, err := New(filepath.ToSlash(resolved), d)
	if err != nil {
		return Rules{}, err
	}
	var claims []claim
	for _, c := range r.claims {
		base := filepath.FromSlash("/" + strings.Join(c.pattern.literal, "/"))
		at, err := ResolveExisting(base)
		if err != nil {
			return Rules{}, err
		}
		for _, form := range []string{r.written(base), at} {
			moved := c
			moved.pattern.literal = parts(filepath.ToSlash(form))
			if !holds(claims, moved) {
				claims = append(claims, moved)
			}
		}
	}
	r.claims = claims
	return r, nil
}

func holds(claims []claim, c claim) bool {
	for _, o := range claims {
		if o.area.Name == c.area.Name && strings.Join(o.pattern.literal, "/") == strings.Join(c.pattern.literal, "/") &&
			strings.Join(o.pattern.glob, "/") == strings.Join(c.pattern.glob, "/") && o.pattern.dir == c.pattern.dir {
			return true
		}
	}
	return false
}

// CheckPath checks abs where it is written and where it leads: refused if either
// is, so a symlink neither opens what its target's table closes nor takes a path
// out of the service it sits in. The root of r must be resolved.
func (r Rules) CheckPath(mode Mode, abs string, granted []string) (Verdict, []string, string, error) {
	abs = filepath.Clean(abs)
	resolved, err := ResolveExisting(abs)
	if err != nil {
		return Refused, nil, "", err
	}
	verdict := Unclaimed
	for _, p := range []string{r.written(abs), resolved} {
		switch v, ways, reason := r.Check(mode, filepath.ToSlash(p), granted); v {
		case Refused:
			return v, ways, reason, nil
		case Allowed:
			verdict = Allowed
		}
	}
	return verdict, nil, "", nil
}

// written follows only the symlinks that lead to the root, so a path keeps the
// table of the service it sits in.
func (r Rules) written(p string) string {
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	dir := r.written(parent)
	at := filepath.Join(dir, listed(dir, filepath.Base(p)))
	if _, below := r.within(parts(filepath.ToSlash(dir))); below {
		return at
	}
	if fi, err := os.Lstat(at); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return at
	}
	if resolved, err := ResolveExisting(at); err == nil {
		if leads := parts(filepath.ToSlash(resolved)); len(leads) <= len(r.root) && equalPrefix(r.root, leads) {
			return resolved
		}
	}
	return at
}

func equalPrefix(full, prefix []string) bool {
	for i := range prefix {
		if full[i] != prefix[i] {
			return false
		}
	}
	return true
}

// ResolveExisting resolves the part of p that exists, spelled as on disk, and
// keeps the rest as written.
func ResolveExisting(p string) (string, error) {
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(append([]string{onDisk(resolved)}, tail...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(append([]string{p}, tail...)...), nil
		}
		tail = append([]string{filepath.Base(p)}, tail...)
		p = parent
	}
}

func onDisk(p string) string {
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	dir := onDisk(parent)
	return filepath.Join(dir, listed(dir, filepath.Base(p)))
}

// A case-insensitive filesystem finds a name under any spelling; only the
// listing has the one the rules compare against.
func listed(dir, name string) string {
	if _, err := os.Lstat(filepath.Join(dir, name)); err != nil {
		return name
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return name
	}
	spelled := name
	for _, e := range entries {
		if e.Name() == name {
			return name
		}
		if strings.EqualFold(e.Name(), name) {
			spelled = e.Name()
		}
	}
	return spelled
}
