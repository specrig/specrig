// Package guard checks a path against one service's rules, rules/1: whether the
// mode on allows editing it, and if not, which mode or grant would.
package guard

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Document is rules/1 as a stack's tool prints it.
type Document struct {
	Specrig string  `json:"specrig"`
	Modes   []Mode  `json:"modes"`
	Classes []Class `json:"classes"`
	Areas   []Area  `json:"areas"`
}

type Mode struct {
	Name     string   `json:"name"`
	Opens    []string `json:"opens"`
	Includes []string `json:"includes,omitempty"`
	Offered  *bool    `json:"offered,omitempty"`
	Allows   string   `json:"allows"`
	Brief    string   `json:"brief"`
}

type Class struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns,omitempty"`
}

// Area is a named set of paths. Nil Modes leaves the class table in charge, so a
// grant only adds; empty Modes closes the paths in every mode.
type Area struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
	Modes []string `json:"modes"`
	Grant []string `json:"grant,omitempty"`
}

func Parse(body []byte) (Document, error) {
	var d Document
	if err := json.Unmarshal(body, &d); err != nil {
		return Document{}, err
	}
	if d.Specrig != "rules/1" {
		return Document{}, fmt.Errorf("the rules are %q, and this guard reads rules/1", d.Specrig)
	}
	return d, nil
}

type Verdict int

const (
	Allowed Verdict = iota
	Refused
	Unclaimed // outside the service, and no area of it names the path
)

func (v Verdict) String() string {
	return [...]string{"allowed", "refused", "unclaimed"}[v]
}

type claim struct {
	area    Area
	pattern pattern
}

type kind struct {
	class    string
	patterns []pattern
}

// Rules answer for one service, whose root is an absolute, clean path.
type Rules struct {
	root   []string
	modes  []Mode
	kinds  []kind
	claims []claim
	Areas  []Area
}

func New(root string, d Document) (Rules, error) {
	r := Rules{root: parts(root), modes: d.Modes, Areas: d.Areas}
	for _, m := range d.Modes {
		for _, c := range m.Opens {
			if !slices.ContainsFunc(d.Classes, func(k Class) bool { return k.Name == c }) {
				return Rules{}, fmt.Errorf("%s mode opens a class %q the rules do not have", m.Name, c)
			}
		}
		for _, other := range m.Includes {
			if _, err := r.Mode(other); err != nil {
				return Rules{}, fmt.Errorf("%s mode includes %w", m.Name, err)
			}
		}
	}
	for _, c := range d.Classes {
		k := kind{class: c.Name}
		for _, p := range c.Patterns {
			k.patterns = append(k.patterns, compile(r.root, p))
		}
		r.kinds = append(r.kinds, k)
	}
	for _, a := range d.Areas {
		for _, m := range append(slices.Clone(a.Modes), a.Grant...) {
			if _, err := r.Mode(m); err != nil {
				return Rules{}, fmt.Errorf("agents.areas.%s: %w", a.Name, err)
			}
		}
		for _, p := range a.Paths {
			r.claims = append(r.claims, claim{area: a, pattern: compile(r.root, p)})
		}
	}
	return r, nil
}

// Modes are the rules', in their order.
func (r Rules) Modes() []Mode { return slices.Clone(r.modes) }

func (r Rules) Mode(name string) (Mode, error) {
	for _, m := range r.modes {
		if m.Name == name {
			return m, nil
		}
	}
	names := make([]string, 0, len(r.modes))
	for _, m := range r.modes {
		names = append(names, m.Name)
	}
	return Mode{}, fmt.Errorf("unknown mode %q (have: %s)", name, strings.Join(names, ", "))
}

func (m Mode) opens(class string) bool { return slices.Contains(m.Opens, class) }

func (m Mode) actsAs(name string) bool { return m.Name == name || slices.Contains(m.Includes, name) }

func (m Mode) offered() bool { return m.Offered == nil || *m.Offered }

func (r Rules) offered(name string) bool {
	m, err := r.Mode(name)
	return err == nil && m.offered()
}

// Check is lexical: abs is absolute and clean, as written or as resolved.
func (r Rules) Check(mode Mode, abs string, granted []string) (Verdict, []string, string) {
	names := parts(abs)
	rel, inside := r.within(names)
	shown := rel
	if !inside {
		shown = filepath.FromSlash(abs)
	}

	class := ""
	if inside {
		class = r.class(names)
	}
	switch class {
	case "humans":
		return Refused, []string{}, shown + " belongs to the humans who own the service; no mode allows it"
	case "generated":
		return Refused, []string{}, shown + " is generated: the stack's tool writes it, in any mode, and never by hand"
	}

	a, claimed := r.area(names)
	if !claimed && !inside {
		return Unclaimed, nil, ""
	}
	if claimed && slices.ContainsFunc(a.Grant, mode.actsAs) && slices.Contains(granted, a.Name) {
		return Allowed, nil, ""
	}

	ways := []string{}
	switch {
	case claimed && a.Modes != nil:
		if slices.ContainsFunc(a.Modes, mode.actsAs) {
			return Allowed, nil, ""
		}
		for _, m := range a.Modes {
			if m != mode.Name && r.offered(m) {
				ways = append(ways, m+" mode")
			}
		}
	case inside:
		if mode.opens(class) {
			return Allowed, nil, ""
		}
		for _, other := range r.modes {
			if other.offered() && other.Name != mode.Name && other.opens(class) {
				ways = append(ways, other.Name+" mode")
			}
		}
	}
	if claimed {
		for _, m := range a.Grant {
			if mode.actsAs(m) {
				m = mode.Name
			} else if !r.offered(m) {
				continue
			}
			if way := fmt.Sprintf("the %s grant in %s mode", a.Name, m); !slices.Contains(ways, way) {
				ways = append(ways, way)
			}
		}
	}
	if len(ways) == 0 {
		return Refused, ways, fmt.Sprintf("%s is in the %s area, which no mode opens", shown, a.Name)
	}
	return Refused, ways, fmt.Sprintf("%s mode does not allow %s; %s does — stop and ask the human for it",
		mode.Name, shown, strings.Join(ways, ", or "))
}

func (r Rules) within(names []string) (string, bool) {
	if len(names) < len(r.root) || !slices.Equal(names[:len(r.root)], r.root) {
		return "", false
	}
	return path.Join(append([]string{"."}, names[len(r.root):]...)...), true
}

func (r Rules) class(names []string) string {
	for _, k := range r.kinds {
		for _, p := range k.patterns {
			if p.matches(names) {
				return k.class
			}
		}
	}
	return "production"
}

func (r Rules) area(names []string) (Area, bool) {
	var won *claim
	for i, c := range r.claims {
		if c.pattern.matches(names) && (won == nil || c.outranks(*won)) {
			won = &r.claims[i]
		}
	}
	if won == nil {
		return Area{}, false
	}
	return won.area, true
}

func (c claim) outranks(o claim) bool {
	if c.pattern.outranks(o.pattern) {
		return true
	}
	if o.pattern.outranks(c.pattern) {
		return false
	}
	return c.area.Name < o.area.Name
}

// Claimed are the paths of a that another area does not name alike and decide.
func (r Rules) Claimed(a Area) []string {
	var paths []string
	for _, p := range a.Paths {
		mine := claim{area: a, pattern: compile(r.root, p)}
		if !slices.ContainsFunc(r.claims, func(o claim) bool {
			return slices.Equal(o.pattern.literal, mine.pattern.literal) && slices.Equal(o.pattern.glob, mine.pattern.glob) && o.outranks(mine)
		}) {
			paths = append(paths, p)
		}
	}
	return paths
}
