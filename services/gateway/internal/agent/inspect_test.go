package agent

import (
	"context"
	"strings"
	"testing"
)

func TestInspectHTML_CleanPage(t *testing.T) {
	html := `<!doctype html><html lang="en"><head>
<meta name="viewport" content="width=device-width">
<title>Hello</title>
<link rel="stylesheet" href="styles.css">
</head><body>
<h1 id="top">Hi</h1>
<a href="#top">back to top</a>
<img src="logo.png" alt="the logo">
</body></html>`
	exists := func(ref string) bool { return ref == "styles.css" || ref == "logo.png" }
	if f := inspectHTML(html, exists); len(f) != 0 {
		t.Errorf("clean page should have no findings, got %v", f)
	}
}

func TestInspectHTML_CatchesDefects(t *testing.T) {
	html := `<html><head>
<link rel="stylesheet" href="missing.css">
<script src="app.js"></script>
</head><body>
<img src="hero.png">
<a href="#nowhere">jump</a>
</body></html>`
	exists := func(ref string) bool { return false } // nothing resolves
	findings := inspectHTML(html, exists)
	joined := strings.Join(findings, "\n")

	wants := []string{
		"no <title>",
		"lang",
		"viewport",
		`stylesheet "missing.css"`,
		`script "app.js"`,
		`image "hero.png"`, // missing file
		"no alt text",
		"#nowhere",
	}
	for _, w := range wants {
		if !strings.Contains(joined, w) {
			t.Errorf("expected a finding containing %q, got:\n%s", w, joined)
		}
	}
}

func TestInspectHTML_ExternalRefsNotFlagged(t *testing.T) {
	html := `<html lang="en"><head><meta name="viewport" content="x"><title>t</title>
<link rel="stylesheet" href="https://cdn.example.com/x.css">
<script src="https://cdn.example.com/x.js"></script>
</head><body><img src="data:image/png;base64,AAAA" alt="ok"><a href="#">noop</a></body></html>`
	exists := func(ref string) bool { return false }
	if f := inspectHTML(html, exists); len(f) != 0 {
		t.Errorf("external/data refs and href=# should not be flagged, got %v", f)
	}
}

func TestWorkspace_InspectPageAndAll(t *testing.T) {
	ws := newTestWS(t)
	// index.html links a stylesheet that IS present and one that isn't.
	_ = ws.WriteFile("styles.css", "body{}")
	_ = ws.WriteFile("index.html", `<html lang="en"><head><meta name="viewport" content="x"><title>Home</title>
<link rel="stylesheet" href="styles.css">
<link rel="stylesheet" href="theme.css">
</head><body><h1>Home</h1></body></html>`)

	report, err := ws.InspectPage("index.html")
	if err != nil {
		t.Fatalf("InspectPage: %v", err)
	}
	if !strings.Contains(report, `theme.css`) || strings.Contains(report, `"styles.css" is linked but not`) {
		t.Errorf("should flag only the missing stylesheet, got:\n%s", report)
	}

	all := ws.InspectAllHTML()
	if !strings.Contains(all, "index.html") {
		t.Errorf("InspectAllHTML should cover index.html, got:\n%s", all)
	}

	// No HTML at all → empty combined report.
	empty := newTestWS(t)
	_ = empty.WriteFile("notes.txt", "hi")
	if got := empty.InspectAllHTML(); got != "" {
		t.Errorf("no HTML files should give an empty report, got %q", got)
	}
}

func TestExecTool_InspectPage(t *testing.T) {
	ws := newTestWS(t)
	_ = ws.WriteFile("p.html", `<html><body><img src="x.png"></body></html>`)
	got := execTool(context.Background(), ws, &toolCall{Tool: "inspect_page", Args: map[string]any{"path": "p.html"}})
	if !strings.Contains(got, "alt") || !strings.Contains(got, "x.png") {
		t.Errorf("inspect_page via execTool should report the defects, got:\n%s", got)
	}
}
