package agentdef

import (
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/jinho-yoo-jack/devsquad/internal/pathguard"
)

// A conservative ownership check: glob roots must be disjoint. This also rejects
// a repeated writer agent, whose stages otherwise share the same files.
func checkWriteConflicts(d Definition) error {
	for i, s := range d.Pipeline.Stages {
		a := d.Agents[s.Agent]
		for _, p := range a.WritePaths {
			if !doublestar.ValidatePattern(p) || pathguard.Protected(p) {
				return fmt.Errorf("invalid write_path: %s", p)
			}
		}
		for _, other := range d.Pipeline.Stages[i+1:] {
			for _, p := range a.WritePaths {
				for _, q := range d.Agents[other.Agent].WritePaths {
					x, y := literalRoot(p), literalRoot(q)
					if x == "" || y == "" || x == y || strings.HasPrefix(x, y+"/") || strings.HasPrefix(y, x+"/") {
						return fmt.Errorf("write_paths conflict: %s and %s (%s, %s)", s.ID, other.ID, p, q)
					}
				}
			}
		}
	}
	return nil
}
func literalRoot(p string) string {
	parts := strings.Split(p, "/")
	out := []string{}
	for _, s := range parts {
		if strings.ContainsAny(s, "*?[{\\") {
			break
		}
		out = append(out, s)
	}
	return strings.Join(out, "/")
}
