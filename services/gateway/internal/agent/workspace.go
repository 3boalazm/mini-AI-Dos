package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// maxFileBytes bounds a single file read or write — generated web
	// files are small; a larger blob is a mistake, and unbounded reads
	// would blow the model's context on the next tool turn.
	maxFileBytes = 256 * 1024
	// maxFilesInWorkspace caps how many files one run can create — a
	// runaway loop writing thousands of files is contained.
	maxFilesInWorkspace = 200
)

// Workspace is one agent run's isolated file tree, rooted under the
// engine's base directory at <base>/<runID>. Every path operation is
// confined to this root: a tool call cannot read or write anything
// outside it, no matter what the model emits.
type Workspace struct {
	root string
}

// newWorkspace creates <base>/<runID> and returns a handle to it.
func newWorkspace(base, runID string) (*Workspace, error) {
	root := filepath.Join(base, runID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &Workspace{root: root}, nil
}

// resolve turns a caller-supplied relative path into an absolute path
// guaranteed to live inside the workspace. It rejects absolute paths
// and anything that would escape via "..". This is the one security
// boundary of the whole file-tool system.
func (w *Workspace) resolve(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	// Reject absolute paths in a way that holds on every OS: filepath's
	// own check, a leading separator (a bare "/etc/x" is NOT absolute on
	// Windows), and any volume name ("C:"). The gateway runs on Linux
	// but dev is Windows, so both must be caught.
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("path must be relative")
	}
	// Reject any parent-dir component outright — clearer and safer than
	// silently cleaning ".." away.
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".." {
			return "", fmt.Errorf("path must not contain '..'")
		}
	}
	full := filepath.Join(w.root, filepath.FromSlash(rel))
	// Independent belt-and-suspenders check that the result is inside.
	check, err := filepath.Rel(w.root, full)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the workspace")
	}
	return full, nil
}

// ReadFile returns a file's contents, bounded by maxFileBytes.
func (w *Workspace) ReadFile(rel string) (string, error) {
	full, err := w.resolve(rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", rel)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", rel)
	}
	if info.Size() > maxFileBytes {
		return "", fmt.Errorf("file too large to read: %s", rel)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WriteFile creates or overwrites a file, making parent directories as
// needed. The file-count cap is enforced for new files only.
func (w *Workspace) WriteFile(rel, content string) error {
	if len(content) > maxFileBytes {
		return fmt.Errorf("content exceeds the %d byte limit", maxFileBytes)
	}
	full, err := w.resolve(rel)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(full); os.IsNotExist(statErr) {
		if files, _ := w.List(); len(files) >= maxFilesInWorkspace {
			return fmt.Errorf("workspace file limit (%d) reached", maxFilesInWorkspace)
		}
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

// EditFile replaces every occurrence of find with replace in a file,
// returning how many were replaced. Missing text is an error so the
// model learns the edit did not apply.
func (w *Workspace) EditFile(rel, find, replace string) (int, error) {
	if find == "" {
		return 0, fmt.Errorf("find text is required")
	}
	content, err := w.ReadFile(rel)
	if err != nil {
		return 0, err
	}
	n := strings.Count(content, find)
	if n == 0 {
		return 0, fmt.Errorf("text not found in %s", rel)
	}
	if err := w.WriteFile(rel, strings.ReplaceAll(content, find, replace)); err != nil {
		return 0, err
	}
	return n, nil
}

// List returns every file in the workspace as slash-separated relative
// paths, sorted — the file tree the UI shows and inspection reads.
func (w *Workspace) List() ([]string, error) {
	var out []string
	err := filepath.WalkDir(w.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(w.root, p)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Search finds lines containing query (case-insensitive) across all
// files, returned as "path:line: snippet", capped at 50 hits.
func (w *Workspace) Search(query string) ([]string, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	files, err := w.List()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	var hits []string
	for _, f := range files {
		content, readErr := w.ReadFile(f)
		if readErr != nil {
			continue
		}
		for i, line := range strings.Split(content, "\n") {
			if strings.Contains(strings.ToLower(line), needle) {
				snippet := strings.TrimSpace(line)
				if len(snippet) > 120 {
					snippet = snippet[:120]
				}
				hits = append(hits, fmt.Sprintf("%s:%d: %s", f, i+1, snippet))
				if len(hits) >= 50 {
					return hits, nil
				}
			}
		}
	}
	return hits, nil
}

// Dump concatenates every file (header + contents) up to limit bytes —
// the deliverable snapshot the inspection stage reviews.
func (w *Workspace) Dump(limit int) string {
	files, _ := w.List()
	var b strings.Builder
	for _, f := range files {
		c, err := w.ReadFile(f)
		if err != nil {
			continue
		}
		header := "--- " + f + " ---\n"
		if b.Len()+len(header) >= limit {
			break
		}
		b.WriteString(header)
		if remaining := limit - b.Len(); len(c) > remaining {
			b.WriteString(c[:remaining])
			b.WriteString("\n...[truncated]\n")
			break
		}
		b.WriteString(c + "\n")
	}
	if b.Len() == 0 {
		return "(empty workspace)"
	}
	return b.String()
}

// Remove deletes the whole workspace tree.
func (w *Workspace) Remove() error {
	return os.RemoveAll(w.root)
}
