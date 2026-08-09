package agent

import (
	"strings"
	"testing"
)

func TestDiffSnapshots(t *testing.T) {
	before := map[string]string{
		"index.html": "line1\nline2\nline3",
		"old.txt":    "gone",
	}
	after := map[string]string{
		"index.html": "line1\nline2 changed\nline3\nline4", // 2 added, 1 removed
		"new.css":    "a\nb",                               // added
	}
	changes := diffSnapshots(before, after)

	got := map[string]FileChange{}
	for _, c := range changes {
		got[c.Path] = c
	}
	if got["new.css"].Status != "added" || got["new.css"].Added != 2 {
		t.Errorf("new.css: %+v", got["new.css"])
	}
	if got["old.txt"].Status != "deleted" {
		t.Errorf("old.txt should be deleted: %+v", got["old.txt"])
	}
	if got["index.html"].Status != "modified" {
		t.Errorf("index.html should be modified: %+v", got["index.html"])
	}
	if got["index.html"].Added != 2 || got["index.html"].Removed != 1 {
		t.Errorf("index.html line counts: got +%d/-%d, want +2/-1", got["index.html"].Added, got["index.html"].Removed)
	}
}

func TestDiffSnapshots_NoChange(t *testing.T) {
	snap := map[string]string{"a.txt": "same"}
	if c := diffSnapshots(snap, snap); len(c) != 0 {
		t.Errorf("identical snapshots should report no changes, got %v", c)
	}
}

func TestUnifiedDiff(t *testing.T) {
	before := map[string]string{"a.txt": "keep\nremove"}
	after := map[string]string{"a.txt": "keep\nadd"}
	d := unifiedDiff(before, after)
	if !strings.Contains(d, "- remove") || !strings.Contains(d, "+ add") || !strings.Contains(d, "  keep") {
		t.Errorf("unified diff wrong:\n%s", d)
	}
}

func TestWorkspace_SnapshotAndRestore(t *testing.T) {
	ws := newTestWS(t)
	_ = ws.WriteFile("index.html", "v1")
	snap := ws.Snapshot()

	// Mutate: change a file and add a new one.
	_ = ws.WriteFile("index.html", "v2-changed")
	_ = ws.WriteFile("extra.css", "added later")

	if err := ws.Restore(snap); err != nil {
		t.Fatalf("restore: %v", err)
	}
	// index.html back to v1, extra.css removed.
	if c, _ := ws.ReadFile("index.html"); c != "v1" {
		t.Errorf("index.html not restored: %q", c)
	}
	if _, err := ws.ReadFile("extra.css"); err == nil {
		t.Error("extra.css should have been removed on restore")
	}
	files, _ := ws.List()
	if len(files) != 1 {
		t.Errorf("workspace should have exactly the snapshot's files, got %v", files)
	}
}

func TestEngine_CompareAndRevert(t *testing.T) {
	// Build writes index.html; inspection fails; fix edits index.html and
	// adds a file. Compare should show the fix's changes; revert undoes them.
	p := &scriptedProvider{responses: []string{
		`["build"]`,
		`{"tool":"write_file","args":{"path":"index.html","content":"<h1>hi</h1>"}}`,
		`{"tool":"done","args":{"summary":"built"}}`,
		"needs a stylesheet", // inspect fails → fix runs
		`{"tool":"write_file","args":{"path":"index.html","content":"<h1>hi</h1>\n<link rel=stylesheet href=s.css>"}}`,
		`{"tool":"write_file","args":{"path":"s.css","content":"body{}"}}`,
		`{"tool":"done","args":{"summary":"added styles"}}`,
	}}
	e := NewEngine(p, "m", t.TempDir(), testLog())
	run, _ := e.Start("build a page", true)
	final := waitFor(t, e, run.ID, StatusCompleted)

	if len(final.Changes) == 0 {
		t.Fatal("fix stage changed files but Changes is empty")
	}
	diff, known := e.Compare(run.ID)
	if !known || !strings.Contains(diff, "s.css") {
		t.Errorf("compare should show the added s.css, got:\n%s", diff)
	}

	if !e.Revert(run.ID) {
		t.Fatal("revert should succeed")
	}
	// After revert: s.css gone, index.html back to the build version.
	if _, _, err := e.ReadRunFile(run.ID, "s.css"); err == nil {
		t.Error("s.css should be gone after revert")
	}
	c, _, _ := e.ReadRunFile(run.ID, "index.html")
	if c != "<h1>hi</h1>" {
		t.Errorf("index.html should be the build version after revert, got %q", c)
	}
	if snap := e.Get(run.ID); len(snap.Changes) != 0 {
		t.Error("Changes should be cleared after revert")
	}
}

func TestEngine_RevertUnknown(t *testing.T) {
	e := NewEngine(&scriptedProvider{responses: []string{"x"}}, "m", t.TempDir(), testLog())
	if e.Revert("run_nope") {
		t.Error("reverting an unknown run should return false")
	}
}
