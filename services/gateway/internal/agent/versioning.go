package agent

import (
	"fmt"
	"os"
	"strings"
)

// Phase A10 "versioning": pure-Go workspace snapshots and diffs, no git
// binary (the alpine runtime has none). The loop snapshots the
// workspace right after the initial build, so once the fix stage edits
// files we can show exactly what changed (added / modified with line
// counts), let the user compare (a unified diff), and revert the fixes.
// This is the foundation real project versioning (A6) will build on.

// FileChange is one entry in a run's change summary.
type FileChange struct {
	Path    string `json:"path"`
	Status  string `json:"status"` // added | modified | deleted
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// diffLineCap bounds line-level diffing; past it a file is reported as
// modified without +/- counts, so a pathological input can't make the
// O(n*m) LCS blow up.
const diffLineCap = 2000

// Snapshot captures the whole workspace as path→content.
func (w *Workspace) Snapshot() map[string]string {
	snap := map[string]string{}
	files, _ := w.List()
	for _, f := range files {
		if c, err := w.ReadFile(f); err == nil {
			snap[f] = c
		}
	}
	return snap
}

// Restore makes the workspace match snap: rewrites every file in snap
// and deletes files that aren't in it. Used by revert — a trusted
// engine operation, unlike the agent's tools (which have no delete).
func (w *Workspace) Restore(snap map[string]string) error {
	current, _ := w.List()
	for _, f := range current {
		if _, keep := snap[f]; !keep {
			if err := w.removeFile(f); err != nil {
				return err
			}
		}
	}
	for path, content := range snap {
		if err := w.WriteFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// removeFile deletes one workspace file (path-checked). Internal to
// versioning; the agent has no delete tool by design.
func (w *Workspace) removeFile(rel string) error {
	full, err := w.resolve(rel)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

// diffSnapshots summarizes the change from before to after.
func diffSnapshots(before, after map[string]string) []FileChange {
	var changes []FileChange
	for path, newContent := range after {
		oldContent, existed := before[path]
		if !existed {
			changes = append(changes, FileChange{Path: path, Status: "added", Added: lineCount(newContent)})
			continue
		}
		if oldContent != newContent {
			added, removed := lineDelta(oldContent, newContent)
			changes = append(changes, FileChange{Path: path, Status: "modified", Added: added, Removed: removed})
		}
	}
	for path, oldContent := range before {
		if _, still := after[path]; !still {
			changes = append(changes, FileChange{Path: path, Status: "deleted", Removed: lineCount(oldContent)})
		}
	}
	return changes
}

// unifiedDiff renders a compact unified diff across all changed files,
// for the compare view.
func unifiedDiff(before, after map[string]string) string {
	var b strings.Builder
	for _, ch := range diffSnapshots(before, after) {
		fmt.Fprintf(&b, "--- %s (%s)\n", ch.Path, ch.Status)
		switch ch.Status {
		case "added":
			for _, ln := range splitLines(after[ch.Path]) {
				b.WriteString("+ " + ln + "\n")
			}
		case "deleted":
			for _, ln := range splitLines(before[ch.Path]) {
				b.WriteString("- " + ln + "\n")
			}
		case "modified":
			b.WriteString(lineDiff(before[ch.Path], after[ch.Path]))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return len(splitLines(s))
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}

// lineDelta returns added/removed line counts between two texts using an
// LCS, falling back to a size-based estimate for very large files.
func lineDelta(oldText, newText string) (added, removed int) {
	a, bb := splitLines(oldText), splitLines(newText)
	if len(a)+len(bb) > diffLineCap {
		if len(bb) > len(a) {
			return len(bb) - len(a), 0
		}
		return 0, len(a) - len(bb)
	}
	lcs := lcsLen(a, bb)
	return len(bb) - lcs, len(a) - lcs
}

// lineDiff renders a +/- unified body for a modified file.
func lineDiff(oldText, newText string) string {
	a, bb := splitLines(oldText), splitLines(newText)
	if len(a)+len(bb) > diffLineCap {
		return "  (large file — showing summary only)\n"
	}
	// Reconstruct a simple diff from the LCS table.
	m, n := len(a), len(bb)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == bb[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var out strings.Builder
	i, j := 0, 0
	for i < m && j < n {
		if a[i] == bb[j] {
			out.WriteString("  " + a[i] + "\n")
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out.WriteString("- " + a[i] + "\n")
			i++
		} else {
			out.WriteString("+ " + bb[j] + "\n")
			j++
		}
	}
	for ; i < m; i++ {
		out.WriteString("- " + a[i] + "\n")
	}
	for ; j < n; j++ {
		out.WriteString("+ " + bb[j] + "\n")
	}
	return out.String()
}

// lcsLen is the length of the longest common subsequence of two lists.
func lcsLen(a, b []string) int {
	m, n := len(a), len(b)
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[n]
}
