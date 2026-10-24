package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/specrig/specrig/guard"
)

// Guard answers guard/1 for the services of one scope, a session: a service's
// rules are read once in it, and again when its marker changes.
type Guard struct {
	Run   Run
	Cache string // where the rules read are kept, a file a service; "" keeps none
	Scope string
}

type kept struct {
	Scope  string          `json:"scope"`
	Stamp  string          `json:"stamp"`
	Legacy bool            `json:"legacy,omitempty"` // the tool predates rules
	Rules  *guard.Document `json:"rules,omitempty"`
}

func (g Guard) Check(svc, mode, path string, granted []string) (Verdict, string) {
	r, v, text := g.rules(svc)
	if r == nil {
		if v != OK {
			return v, text
		}
		args := []string{"guard", mode, path}
		for _, a := range granted {
			args = append(args, "--grant", a)
		}
		return classify(g.Run(svc, args...))
	}
	m, err := r.Mode(mode)
	if err != nil {
		return Refused, err.Error()
	}
	verdict, _, reason, err := r.CheckPath(m, path, granted)
	switch {
	case err != nil:
		return Broken, err.Error()
	case verdict == guard.Refused:
		return Refused, reason
	case verdict == guard.Unclaimed:
		return Unclaimed, "unclaimed: no area of this service names " + path
	}
	return OK, ""
}

func (g Guard) Mode(svc, mode string) (Verdict, string) {
	r, v, text := g.rules(svc)
	if r == nil {
		if v != OK {
			return v, text
		}
		return classify(g.Run(svc, "mode", mode))
	}
	m, err := r.Mode(mode)
	if err != nil {
		return Refused, err.Error()
	}
	return OK, guard.Brief(m)
}

// Grant with no area lists what mode opens and takes as grants.
func (g Guard) Grant(svc, mode, area string) (Verdict, string) {
	r, v, text := g.rules(svc)
	if r == nil {
		if v != OK {
			return v, text
		}
		args := []string{"grant", mode}
		if area != "" {
			args = append(args, area)
		}
		return classify(g.Run(svc, args...))
	}
	m, err := r.Mode(mode)
	if err != nil {
		return Refused, err.Error()
	}
	if area == "" {
		return OK, r.Grants(m)
	}
	out, err := r.Grant(m, area)
	if err != nil {
		return Refused, err.Error()
	}
	return OK, out
}

// rules are nil with OK for a tool older than them, which is asked instead.
func (g Guard) rules(svc string) (*guard.Rules, Verdict, string) {
	stamp := stamp(svc)
	k, ok := g.kept(svc)
	if !ok || k.Scope != g.Scope || k.Stamp != stamp {
		v, out := classify(g.Run(svc, "rules"))
		switch v {
		case NoModes:
			k = kept{Scope: g.Scope, Stamp: stamp, Legacy: true}
		case OK:
			d, err := guard.Parse([]byte(lastObject(out)))
			if err != nil {
				return nil, Broken, "rules: " + err.Error()
			}
			k = kept{Scope: g.Scope, Stamp: stamp, Rules: &d}
		default:
			return nil, v, out
		}
		g.keep(svc, k)
	}
	if k.Legacy {
		return nil, OK, ""
	}
	r, err := guard.NewOnDisk(svc, *k.Rules)
	if err != nil {
		return nil, Broken, "rules: " + err.Error()
	}
	return &r, OK, ""
}

// The tool's launcher may print before it — a build, a download.
func lastObject(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); strings.HasPrefix(line, "{") {
			return line
		}
	}
	return out
}

// A service's rules come from its tool and its marker; a tool updated in the
// middle of a session is read in the next one.
func stamp(svc string) string {
	var b strings.Builder
	for _, m := range markers {
		if fi, err := os.Stat(filepath.Join(svc, m)); err == nil {
			fmt.Fprintf(&b, "%s %d %d\n", m, fi.Size(), fi.ModTime().UnixNano())
		}
	}
	return b.String()
}

func (g Guard) file(svc string) string {
	sum := sha256.Sum256([]byte(svc))
	return filepath.Join(g.Cache, hex.EncodeToString(sum[:8])+".json")
}

func (g Guard) kept(svc string) (kept, bool) {
	if g.Cache == "" {
		return kept{}, false
	}
	body, err := os.ReadFile(g.file(svc))
	if err != nil {
		return kept{}, false
	}
	var k kept
	if json.Unmarshal(body, &k) != nil || (!k.Legacy && k.Rules == nil) {
		return kept{}, false
	}
	return k, true
}

// Written through a rename: two hooks of one session can run at once.
func (g Guard) keep(svc string, k kept) {
	if g.Cache == "" {
		return
	}
	if err := os.MkdirAll(g.Cache, 0o755); err != nil {
		return
	}
	body, err := json.Marshal(k)
	if err != nil {
		return
	}
	path := g.file(svc)
	tmp := fmt.Sprintf("%s.%d", path, os.Getpid())
	if os.WriteFile(tmp, body, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}
