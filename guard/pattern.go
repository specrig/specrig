package guard

import (
	"path"
	"slices"
	"strings"
)

// A pattern is relative to the service: the names up to its first wildcard, the
// rest as written, and whether it claims everything below.
type pattern struct {
	literal []string
	glob    []string
	dir     bool
}

func compile(root []string, p string) pattern {
	names := strings.Split(path.Clean(p), "/")
	i := slices.IndexFunc(names, func(name string) bool { return strings.ContainsAny(name, `*?[\`) })
	if i < 0 {
		i = len(names)
	}
	literal := parts(path.Join(append([]string{"/" + strings.Join(root, "/")}, names[:i]...)...))
	return pattern{literal: literal, glob: names[i:], dir: strings.HasSuffix(p, "/")}
}

func (p pattern) matches(names []string) bool {
	return len(names) >= len(p.literal) && slices.Equal(names[:len(p.literal)], p.literal) &&
		globbed(p.glob, names[len(p.literal):], p.dir)
}

func globbed(glob, names []string, dir bool) bool {
	if len(glob) == 0 {
		return len(names) == 0 || dir
	}
	if glob[0] == "**" {
		for i := 0; i <= len(names); i++ {
			if globbed(glob[1:], names[i:], dir) {
				return true
			}
		}
		return false
	}
	if len(names) == 0 {
		return false
	}
	if ok, _ := path.Match(glob[0], names[0]); !ok {
		return false
	}
	return globbed(glob[1:], names[1:], dir)
}

func (p pattern) outranks(o pattern) bool {
	if len(p.literal) != len(o.literal) {
		return len(p.literal) > len(o.literal)
	}
	return len(p.glob) > len(o.glob)
}

func parts(abs string) []string {
	return strings.Split(strings.Trim(path.Clean(abs), "/"), "/")
}
