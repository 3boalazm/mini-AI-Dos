package agent

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

// Phase A4 "inspect": the agent sees its own HTML — not pixels (a real
// screenshot needs a headless browser the runtime image doesn't carry)
// but structure. inspect_page reports the page's title/headings/links/
// images and, most usefully, catches broken local references: a linked
// stylesheet that isn't in the workspace, an anchor to an id that
// doesn't exist, an <img> with no alt. That closes the See → Critique
// → Fix loop on concrete, fixable defects everywhere the gateway runs.
//
// Extraction is regex-based rather than a full HTML parser: findings
// are advisory (the model reads them and decides), so approximate
// matching on the fairly clean markup the agent produces is enough and
// adds no dependency.

var (
	reTitle      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reHTMLLang   = regexp.MustCompile(`(?is)<html[^>]*\blang\s*=`)
	reViewport   = regexp.MustCompile(`(?is)<meta[^>]*\bname\s*=\s*["']?viewport`)
	reLinkTag    = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reScriptTag  = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']([^"']+)["'][^>]*>`)
	reImgTag     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reHeadingTag = regexp.MustCompile(`(?is)<h([1-3])\b[^>]*>(.*?)</h[1-3]>`)
	reIDAttr     = regexp.MustCompile(`(?is)\bid\s*=\s*["']([^"']+)["']`)
	reHrefHash   = regexp.MustCompile(`(?is)\bhref\s*=\s*["'](#[^"']*)["']`)
	reAnyHref    = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
)

// attr extracts a single attribute's value from one tag string.
func attr(tag, name string) string {
	re := regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(name) + `\s*=\s*["']([^"']*)["']`)
	if m := re.FindStringSubmatch(tag); m != nil {
		return m[1]
	}
	return ""
}

// isLocalRef reports whether a URL points at a workspace-relative asset
// (something inspect_page can verify exists), as opposed to an external
// URL, a data URI, or a page anchor.
func isLocalRef(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	lower := strings.ToLower(u)
	for _, p := range []string{"http://", "https://", "//", "data:", "mailto:", "tel:", "#", "javascript:"} {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	return true
}

// inspectHTML runs the structural checks on one page. exists reports
// whether a workspace-relative path is present. Returns human-readable
// finding lines; an empty slice means nothing worth flagging.
func inspectHTML(content string, exists func(string) bool) []string {
	var out []string

	if m := reTitle.FindStringSubmatch(content); m == nil {
		out = append(out, "no <title>")
	} else if strings.TrimSpace(m[1]) == "" {
		out = append(out, "<title> is empty")
	}
	if !reHTMLLang.MatchString(content) {
		out = append(out, "<html> has no lang attribute")
	}
	if !reViewport.MatchString(content) {
		out = append(out, "no viewport meta (page won't be mobile-responsive)")
	}

	// Stylesheets: local rel=stylesheet hrefs must resolve.
	for _, tag := range reLinkTag.FindAllString(content, -1) {
		if !strings.Contains(strings.ToLower(tag), "stylesheet") {
			continue
		}
		href := attr(tag, "href")
		if isLocalRef(href) && !exists(href) {
			out = append(out, fmt.Sprintf("stylesheet %q is linked but not in the workspace", href))
		}
	}

	// Scripts: local src must resolve.
	for _, m := range reScriptTag.FindAllStringSubmatch(content, -1) {
		if isLocalRef(m[1]) && !exists(m[1]) {
			out = append(out, fmt.Sprintf("script %q is referenced but not in the workspace", m[1]))
		}
	}

	// Images: local src must resolve; every image should have alt text.
	for _, tag := range reImgTag.FindAllString(content, -1) {
		src := attr(tag, "src")
		if src == "" {
			out = append(out, "an <img> has no src")
		} else if isLocalRef(src) && !exists(src) {
			out = append(out, fmt.Sprintf("image %q is referenced but not in the workspace", src))
		}
		if attr(tag, "alt") == "" {
			label := src
			if label == "" {
				label = "(no src)"
			}
			out = append(out, fmt.Sprintf("image %q has no alt text", label))
		}
	}

	// Anchor links to #ids that don't exist on the page.
	ids := map[string]bool{}
	for _, m := range reIDAttr.FindAllStringSubmatch(content, -1) {
		ids[m[1]] = true
	}
	for _, m := range reHrefHash.FindAllStringSubmatch(content, -1) {
		target := strings.TrimPrefix(m[1], "#")
		if target == "" {
			continue // href="#" is a common no-op, not a defect
		}
		if !ids[target] {
			out = append(out, fmt.Sprintf("link to #%s but no element has that id", target))
		}
	}

	return out
}

// summarizeHTML returns a one-line structural summary (title, counts) to
// prefix a page's findings, so the model has context for the warnings.
func summarizeHTML(content string) string {
	title := ""
	if m := reTitle.FindStringSubmatch(content); m != nil {
		title = strings.TrimSpace(m[1])
	}
	headings := len(reHeadingTag.FindAllString(content, -1))
	links := len(reAnyHref.FindAllString(content, -1))
	imgs := len(reImgTag.FindAllString(content, -1))
	return fmt.Sprintf("title=%q, %d headings, %d links, %d images", title, headings, links, imgs)
}

// InspectPage reads one HTML file and returns its inspection report.
func (w *Workspace) InspectPage(path string) (string, error) {
	content, err := w.ReadFile(path)
	if err != nil {
		return "", err
	}
	return w.reportFor(path, content), nil
}

// InspectAllHTML runs inspection across every .html/.htm file in the
// workspace and returns a combined report, or "" when there are none.
func (w *Workspace) InspectAllHTML() string {
	files, _ := w.List()
	var b strings.Builder
	for _, f := range files {
		lower := strings.ToLower(f)
		if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
			continue
		}
		content, err := w.ReadFile(f)
		if err != nil {
			continue
		}
		b.WriteString(w.reportFor(f, content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// refRelativeTo resolves an asset reference against the directory of the
// HTML file that contains it, in slash space, so "css/app.css" from a
// root page and "../app.css" from a subfolder page both land correctly.
func refRelativeTo(htmlPath, ref string) string {
	return path.Join(path.Dir(htmlPath), ref)
}

// reportFor formats one page's summary and findings.
func (w *Workspace) reportFor(htmlPath, content string) string {
	exists := func(ref string) bool {
		full, err := w.resolve(refRelativeTo(htmlPath, ref))
		if err != nil {
			return false
		}
		_, statErr := os.Stat(full)
		return statErr == nil
	}
	findings := inspectHTML(content, exists)
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", htmlPath, summarizeHTML(content))
	if len(findings) == 0 {
		b.WriteString("  OK: no structural issues found")
		return b.String()
	}
	for _, f := range findings {
		b.WriteString("  ⚠ " + f + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
