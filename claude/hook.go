package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/specrig/specrig/service"
)

const (
	unswitched = "redgreen"
	prefix     = "/specrig:"
)

type hook struct {
	data string
	run  service.Run
	// Temporary directories: a service copied into one is an agent's working
	// copy — nobody reviews or commits it — so edits there are never checked.
	scratch []string
}

func (h hook) guard(session string) service.Guard {
	return service.Guard{Run: h.run, Cache: filepath.Join(h.data, "rules"), Scope: session}
}

func (h hook) inScratch(path string) bool {
	for _, p := range []string{filepath.Clean(path), filepath.Join(service.Resolve(filepath.Dir(path)), filepath.Base(path))} {
		for _, root := range h.scratch {
			if rel, err := filepath.Rel(root, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// One mode holds for every service the task names, and the rest are closed while
// it is on.
type state struct {
	Mode string `json:"mode"`
	// The services the last command named by path. A command naming none acts
	// on them, so a path is typed once per task — there is no completion for it.
	Services []string        `json:"services"`
	Granted  []string        `json:"granted"` // areas granted on top of the mode; a switch clears them
	Warned   map[string]bool `json:"warned"`  // services already told their guard does not answer
}

func (s state) modeOf(svc string) string {
	if s.Mode != "" && slices.Contains(s.Services, svc) {
		return s.Mode
	}
	return unswitched
}

func (h hook) sessionStart(in input) string {
	var b strings.Builder
	b.WriteString("specrig plugin active: Rule #11 modes are enforced for every service a marker names — " +
		"specrig.yaml or waffle.yaml. An edit the mode does not allow is refused, naming the mode or grant that would allow it. " +
		"Only the human switches the mode, by typing /specrig:<mode> [./path/to/service…], " +
		"and only the human grants an area, by typing /specrig:allow <area>; nothing you run does either.\n")
	st := h.load(in.SessionID)
	if st.Mode != "" && len(st.Services) > 0 {
		fmt.Fprintf(&b, "Current mode: %s, for %s.\n", st.Mode, h.names(in.Cwd, st.Services))
	} else {
		fmt.Fprintf(&b, "No mode is switched on: every service is in %s, and no Rule #11 task has started.\n", unswitched)
	}
	if len(st.Granted) > 0 {
		fmt.Fprintf(&b, "Granted: %s.\n", strings.Join(st.Granted, ", "))
	}
	return b.String()
}

func (h hook) prompt(in input) string {
	fields := strings.Fields(in.Prompt)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], prefix) {
		return ""
	}
	command := strings.TrimPrefix(fields[0], prefix)

	var named, rest, empty []string
	for _, arg := range fields[1:] {
		if found, ok := service.Below(in.Cwd, arg); ok {
			if len(found) == 0 {
				empty = append(empty, arg)
			}
			for _, svc := range found {
				if !slices.Contains(named, svc) {
					named = append(named, svc)
				}
			}
		} else if svc := service.Named(in.Cwd, arg); svc != "" {
			named = append(named, svc)
		} else {
			rest = append(rest, arg)
		}
	}
	if len(empty) > 0 {
		return fmt.Sprintf("Nothing was switched: %s names no service below %s.\n",
			strings.Join(empty, ", "), in.Cwd)
	}
	st := h.load(in.SessionID)
	services := named
	switch {
	case len(services) > 0:
	case len(st.Services) > 0:
		services = st.Services
	default:
		if svc := service.Of(in.Cwd); svc != "" {
			services = []string{svc}
		} else {
			return fmt.Sprintf("Nothing was switched: no service is named, none was named earlier in "+
				"this session, and %s is not inside one. Tell the human to name one by its path, once: "+
				"%s%s ./path/to/service.\n", in.Cwd, prefix, command)
		}
	}

	if command == "allow" {
		out := h.allow(in, &st, services, rest)
		h.save(in.SessionID, st)
		return out
	}

	var b strings.Builder
	switched := false
	for _, svc := range services {
		name := display(in.Cwd, svc)
		v, text := h.guard(in.SessionID).Mode(svc, command)
		switch v {
		case service.OK:
			switched = true
			if command == unswitched && st.modeOf(svc) == "spec" {
				text += "\nThis switch is the approval of the specification handed over in spec mode."
			} else if command == unswitched {
				text += "\nThis switch is not an approval: the service was not in spec mode."
			}
			fmt.Fprintf(&b, "%s:\n%s\n\n", name, text)
		case service.NoModes:
			fmt.Fprintf(&b, "%s: its guard has no modes yet, so edits there are not checked (%s).\n\n", name, text)
		default:
			fmt.Fprintf(&b, "%s was not switched: %s\n\n", name, text)
		}
	}
	if !switched {
		return b.String()
	}
	st.Mode, st.Services, st.Granted = command, services, nil
	h.save(in.SessionID, st)
	return fmt.Sprintf("The human switched to %s mode, for %s; every other service is closed while it is on, "+
		"and nothing is granted.\n\n%s", command, h.names(in.Cwd, services), b.String())
}

// allow answers /specrig:allow [area]: with no name it lists the areas the mode
// takes as grants; with one, each service's rules check it belongs there.
func (h hook) allow(in input, st *state, services, rest []string) string {
	mode := st.Mode
	if mode == "" {
		mode = unswitched
	}
	area := ""
	if len(rest) > 0 {
		area = rest[0]
	}
	var b strings.Builder
	granted := false
	for _, svc := range services {
		name := display(in.Cwd, svc)
		v, text := h.guard(in.SessionID).Grant(svc, mode, area)
		switch v {
		case service.OK:
			granted = granted || area != ""
			fmt.Fprintf(&b, "%s, in %s mode:\n%s\n\n", name, mode, text)
		case service.NoModes:
			fmt.Fprintf(&b, "%s: its guard has no grants yet (%s).\n\n", name, text)
		default:
			fmt.Fprintf(&b, "%s: %s\n\n", name, text)
		}
	}
	if !granted {
		return b.String()
	}
	if !slices.Contains(st.Granted, area) {
		st.Granted = append(st.Granted, area)
	}
	return fmt.Sprintf("The human granted %s, on top of %s mode, until the mode changes.\n\n%s",
		area, mode, b.String())
}

type decision struct {
	HookSpecificOutput *denial `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string  `json:"systemMessage,omitempty"`
}

type denial struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// A nil answer defers to the normal permission flow; this hook only ever
// refuses, it never grants.
func (h hook) edit(in input) *decision {
	path := in.ToolInput.FilePath
	if path == "" {
		path = in.ToolInput.NotebookPath
	}
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(in.Cwd, path)
	}
	if h.inScratch(path) {
		return nil
	}
	st := h.load(in.SessionID)

	// Inside a service, that service decides — unless a task is on and does not
	// name it. Outside every service, each one the task names is asked whether one
	// of its areas claims the path.
	asked := st.Services
	if svc := service.Of(filepath.Dir(path)); svc != "" {
		if st.Mode != "" && !slices.Contains(st.Services, svc) {
			return &decision{HookSpecificOutput: &denial{
				HookEventName:      "PreToolUse",
				PermissionDecision: "deny",
				PermissionDecisionReason: fmt.Sprintf("%s is not in this task, which is %s mode for %s — "+
					"stop and ask the human to add it: %s%s with every service the task needs, "+
					"or ./... for all below a directory.", display(in.Cwd, svc), st.Mode, h.names(in.Cwd, st.Services), prefix, st.Mode),
			}}
		}
		asked = []string{svc}
	}
	var warn []string
	for _, svc := range asked {
		switch v, text := h.guard(in.SessionID).Check(svc, st.modeOf(svc), path, st.Granted); v {
		case service.Refused:
			how := ". The human switches with " + prefix + "<mode>" + argument(in.Cwd, svc)
			if strings.Contains(text, " grant ") {
				how += ", and grants with " + prefix + "allow <area>"
			}
			return &decision{HookSpecificOutput: &denial{
				HookEventName:            "PreToolUse",
				PermissionDecision:       "deny",
				PermissionDecisionReason: text + how + ".",
			}}
		case service.OK:
			return nil
		case service.NoModes, service.Broken:
			if !st.Warned[svc] {
				st.Warned[svc] = true
				if v == service.NoModes {
					warn = append(warn, display(in.Cwd, svc)+": its guard has no modes yet, so edits there are not checked ("+text+").")
				} else {
					warn = append(warn, display(in.Cwd, svc)+": its guard did not run ("+text+"), so edits there are not checked.")
				}
			}
		}
	}
	if len(warn) == 0 {
		return nil
	}
	h.save(in.SessionID, st)
	return &decision{SystemMessage: strings.Join(warn, " ")}
}

func (h hook) names(cwd string, services []string) string {
	shown := make([]string, 0, len(services))
	for _, svc := range services {
		shown = append(shown, display(cwd, svc))
	}
	return strings.Join(shown, ", ")
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func (h hook) file(session string) string {
	return filepath.Join(h.data, "sessions", unsafe.ReplaceAllString(session, "_")+".json")
}

func (h hook) load(session string) state {
	st := state{}
	if body, err := os.ReadFile(h.file(session)); err == nil {
		_ = json.Unmarshal(body, &st)
	}
	if st.Warned == nil {
		st.Warned = map[string]bool{}
	}
	return st
}

// Written through a rename: two hooks of one session can run at once.
func (h hook) save(session string, st state) {
	path := h.file(session)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	body, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp := fmt.Sprintf("%s.%d", path, os.Getpid())
	if os.WriteFile(tmp, body, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// What follows /specrig:<mode> to name svc: nothing when the session runs in it.
func argument(cwd, svc string) string {
	rel, err := filepath.Rel(service.Resolve(cwd), svc)
	switch {
	case err != nil, strings.HasPrefix(rel, ".."):
		return " " + svc
	case rel == ".":
		return ""
	default:
		return " ./" + filepath.ToSlash(rel)
	}
}

func display(cwd, svc string) string {
	if rel, err := filepath.Rel(service.Resolve(cwd), svc); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return "this service"
		}
		return rel
	}
	return svc
}
