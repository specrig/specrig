// Command specrig-hook is the Claude Code side of Rule #11's modes. It keeps
// which mode the human switched the services to, per session, and checks every
// edit against each service's rules, which the tool its marker names prints — so
// the rules come from the tool version the service depends on.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/specrig/specrig/service"
)

type input struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Prompt    string `json:"prompt"`
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

func main() {
	if len(os.Args) != 2 {
		// Not 2: for PreToolUse that exit code would block the edit.
		fmt.Fprintln(os.Stderr, "usage: specrig-hook session-start|prompt|edit")
		os.Exit(1)
	}
	var in input
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	h := hook{data: dataDir(), run: service.Tool, scratch: temporaryDirs()}

	switch os.Args[1] {
	case "session-start":
		fmt.Print(h.sessionStart(in))
	case "prompt":
		fmt.Print(h.prompt(in))
	case "edit":
		if out := h.edit(in); out != nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(out)
		}
	}
}

// On macOS /tmp is a link to /private/tmp and TMPDIR sits under /var, a link
// to /private/var, so each is kept as written and as resolved.
func temporaryDirs() []string {
	var dirs []string
	for _, d := range []string{os.TempDir(), "/tmp"} {
		for _, form := range []string{filepath.Clean(d), service.Resolve(d)} {
			if !slices.Contains(dirs, form) {
				dirs = append(dirs, form)
			}
		}
	}
	return dirs
}

func dataDir() string {
	if dir := os.Getenv("CLAUDE_PLUGIN_DATA"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "specrig-plugin")
}
