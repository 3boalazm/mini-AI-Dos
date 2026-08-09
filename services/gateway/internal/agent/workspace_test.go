package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestWS(t *testing.T) *Workspace {
	t.Helper()
	ws, err := newWorkspace(t.TempDir(), "run_test")
	if err != nil {
		t.Fatalf("newWorkspace: %v", err)
	}
	return ws
}

// TestResolve_RejectsEscapes is the security-critical test: no path a
// tool call can name may resolve outside the workspace root.
func TestResolve_RejectsEscapes(t *testing.T) {
	ws := newTestWS(t)
	bad := []string{
		"../secret",
		"../../etc/passwd",
		"a/../../b",
		"foo/../../../bar",
		"/etc/passwd",
		`C:\Windows\System32`,
		"",
		"   ",
	}
	for _, p := range bad {
		if _, err := ws.resolve(p); err == nil {
			t.Errorf("resolve(%q) should have been rejected", p)
		}
	}

	good := []string{"index.html", "src/app.js", "a/b/c.css", "./styles.css"}
	for _, p := range good {
		full, err := ws.resolve(p)
		if err != nil {
			t.Errorf("resolve(%q) should be allowed: %v", p, err)
			continue
		}
		rel, _ := filepath.Rel(ws.root, full)
		if strings.HasPrefix(rel, "..") {
			t.Errorf("resolve(%q) escaped root: %s", p, full)
		}
	}
}

// TestWrite_TraversalIsContained proves an escape attempt writes
// nothing outside the root even though the write "succeeds" (it lands
// on the cleaned, contained path).
func TestWrite_TraversalIsContained(t *testing.T) {
	base := t.TempDir()
	ws, err := newWorkspace(base, "run_x")
	if err != nil {
		t.Fatal(err)
	}
	// A sibling file outside the workspace we must never touch.
	sentinel := filepath.Join(base, "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = ws.WriteFile("../SENTINEL", "hacked") // should be rejected
	got, _ := os.ReadFile(sentinel)
	if string(got) != "original" {
		t.Fatalf("sentinel outside workspace was modified: %q", got)
	}
}

func TestWorkspace_ReadWriteEditListSearch(t *testing.T) {
	ws := newTestWS(t)

	if err := ws.WriteFile("index.html", "<h1>Hello</h1>"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ws.WriteFile("css/styles.css", "body{margin:0}"); err != nil {
		t.Fatalf("write nested: %v", err)
	}

	got, err := ws.ReadFile("index.html")
	if err != nil || got != "<h1>Hello</h1>" {
		t.Fatalf("read: %q err=%v", got, err)
	}

	n, err := ws.EditFile("index.html", "Hello", "Bonjour")
	if err != nil || n != 1 {
		t.Fatalf("edit: n=%d err=%v", n, err)
	}
	if got, _ := ws.ReadFile("index.html"); got != "<h1>Bonjour</h1>" {
		t.Errorf("edit result: %q", got)
	}
	if _, err := ws.EditFile("index.html", "not-present", "x"); err == nil {
		t.Error("editing absent text should error")
	}

	files, _ := ws.List()
	if len(files) != 2 || files[0] != "css/styles.css" || files[1] != "index.html" {
		t.Errorf("list (should be sorted, slash paths): %v", files)
	}

	hits, err := ws.Search("bonjour")
	if err != nil || len(hits) != 1 || !strings.HasPrefix(hits[0], "index.html:1:") {
		t.Errorf("search: %v err=%v", hits, err)
	}
}

func TestWorkspace_ReadMissing(t *testing.T) {
	ws := newTestWS(t)
	if _, err := ws.ReadFile("nope.txt"); err == nil {
		t.Error("reading a missing file should error")
	}
}

func TestWorkspace_WriteSizeLimit(t *testing.T) {
	ws := newTestWS(t)
	big := strings.Repeat("a", maxFileBytes+1)
	if err := ws.WriteFile("big.txt", big); err == nil {
		t.Error("oversized write should be rejected")
	}
}
