package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Run asks the tool the marker in dir names; args are guard/1's.
type Run func(dir string, args ...string) (string, error)

// Tool is the Run that runs the tool.
func Tool(dir string, args ...string) (string, error) {
	argv, err := toolOf(dir, args[0])
	if err != nil {
		return err.Error(), err
	}
	cmd := exec.Command(argv[0], append(argv[1:], args[1:]...)...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	return strings.TrimSpace(out.String()), err
}

type noGuard struct{ command string }

// Worded as a guard that predates the command, which a service with none is.
func (e noGuard) Error() string {
	return fmt.Sprintf("Unknown command %q: %s names no guard", e.command, marker)
}

func toolOf(dir, command string) ([]string, error) {
	body, err := os.ReadFile(filepath.Join(dir, marker))
	if err != nil {
		return nil, err
	}
	argv := guardIn(string(body))
	if len(argv) == 0 {
		return nil, noGuard{command}
	}
	return append(argv, command), nil
}

// guardIn reads the top-level guard key of specrig.yaml: a flow list, a block
// list, or the arguments as one line.
func guardIn(body string) []string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		rest, ok := strings.CutPrefix(line, "guard:")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(uncomment(rest))
		switch {
		case strings.HasPrefix(rest, "["):
			var argv []string
			for item := range strings.SplitSeq(strings.Trim(rest, "[]"), ",") {
				if item = unquote(strings.TrimSpace(item)); item != "" {
					argv = append(argv, item)
				}
			}
			return argv
		case rest == "":
			var argv []string
			for _, next := range lines[i+1:] {
				item, ok := strings.CutPrefix(strings.TrimSpace(next), "- ")
				if !ok {
					break
				}
				argv = append(argv, unquote(strings.TrimSpace(uncomment(item))))
			}
			return argv
		default:
			return strings.Fields(unquote(rest))
		}
	}
	return nil
}

func uncomment(s string) string {
	if i := strings.Index(s, " #"); i >= 0 {
		return s[:i]
	}
	return s
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

type Verdict int

const (
	OK        Verdict = iota
	Unclaimed         // outside the service, and no area of it names the path
	Refused           // the guard answered no, with a reason
	NoModes           // the service has no guard, or one that predates the command
	Broken            // the guard did not run
)

// A guard prints "Error: <reason>" or `Unknown command "x"`, perhaps after its
// launcher's own output — a build, a download; anything else is a failure to run.
func classify(out string, err error) (Verdict, string) {
	lines := strings.Split(out, "\n")
	at := func(prefix string) int {
		return slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, prefix) })
	}
	switch {
	case err == nil:
		if i := at("unclaimed"); i >= 0 {
			return Unclaimed, strings.Join(lines[i:], "\n")
		}
		return OK, out
	case at("Error: ") >= 0:
		return Refused, strings.TrimPrefix(strings.Join(lines[at("Error: "):], "\n"), "Error: ")
	case strings.Contains(out, "Unknown command"):
		return NoModes, out
	case out != "":
		return Broken, out
	default:
		return Broken, err.Error()
	}
}
