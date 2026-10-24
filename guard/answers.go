package guard

import (
	"fmt"
	"slices"
	"strings"
)

// Brief is guard/1's mode answer: what a tool shows the agent when the human
// switches to m.
func Brief(m Mode) string {
	return fmt.Sprintf("Mode: %s\n%s\nMay be edited: %s.", m.Name, m.Brief, m.Allows)
}

// Grant is guard/1's grant answer for one area.
func (r Rules) Grant(m Mode, name string) (string, error) {
	i := slices.IndexFunc(r.Areas, func(a Area) bool { return a.Name == name })
	if i < 0 {
		return "", fmt.Errorf("no area is named %q", name)
	}
	a := r.Areas[i]
	paths := r.Claimed(a)
	switch {
	case len(paths) == 0:
		return "", fmt.Errorf("the %s area opens nothing: another area names each of its paths alike and decides there", a.Name)
	case slices.ContainsFunc(a.Modes, m.actsAs):
		return "", fmt.Errorf("the %s area is open in %s mode already and takes no grant there", a.Name, m.Name)
	case !slices.ContainsFunc(a.Grant, m.actsAs):
		in := "no mode"
		if len(a.Grant) > 0 {
			in = strings.Join(a.Grant, ", ")
		}
		return "", fmt.Errorf("the %s area is not granted in %s mode (it is in: %s)", a.Name, m.Name, in)
	}
	return fmt.Sprintf("Grant: %s, on top of %s mode, until the mode changes.\nMay also be edited: %s.",
		a.Name, m.Name, strings.Join(paths, ", ")), nil
}

// Grants is guard/1's grant answer with no area: the areas m opens on their own,
// and those it takes as a grant.
func (r Rules) Grants(m Mode) string {
	var open, grants []string
	for _, a := range r.Areas {
		paths := r.Claimed(a)
		if len(paths) == 0 {
			continue
		}
		line := fmt.Sprintf("  %s — %s", a.Name, strings.Join(paths, ", "))
		if slices.ContainsFunc(a.Modes, m.actsAs) {
			open = append(open, line)
		}
		if slices.ContainsFunc(a.Grant, m.actsAs) {
			grants = append(grants, line)
		}
	}
	var lines []string
	if len(open) > 0 {
		lines = append(append(lines, fmt.Sprintf("Open in %s mode on their own:", m.Name)), open...)
	}
	if len(grants) > 0 {
		lines = append(append(lines, fmt.Sprintf("Grants in %s mode:", m.Name)), grants...)
	} else {
		lines = append(lines, fmt.Sprintf("No area takes a grant in %s mode.", m.Name))
	}
	return strings.Join(lines, "\n")
}
