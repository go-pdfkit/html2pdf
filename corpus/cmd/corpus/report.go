package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const analysisMarker = "<!-- BEGIN ANALYSIS -->"

// writeReport renders results as a Markdown table, preserving any hand-written
// analysis below analysisMarker from a previous run — same convention as
// engine-webengine/bench's REPORT.md, so a re-run doesn't clobber notes.
func writeReport(path string, results []result) error {
	var preserved string
	if old, err := os.ReadFile(path); err == nil {
		if i := strings.Index(string(old), analysisMarker); i >= 0 {
			preserved = string(old)[i:]
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# html2pdf corpus run — %s\n\n", time.Now().UTC().Format("2006-01-02"))

	ok, fail := 0, 0
	for _, r := range results {
		if r.OK {
			ok++
		} else {
			fail++
		}
	}
	fmt.Fprintf(&b, "%d/%d succeeded.\n\n", ok, len(results))

	b.WriteString("| URL | Status | Pages | PDF | Text chars | Fetch | Render |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, r := range results {
		status := "✅"
		if !r.OK {
			status = "❌ " + r.Error
		}
		fmt.Fprintf(&b, "| [%s](%s) | %s | %d | %d B | %d | %dms | %dms |\n",
			r.URL, r.URL, status, r.Pages, r.PDFBytes, r.TextChars, r.FetchMs, r.RenderMs)
	}
	b.WriteString("\n")

	if preserved != "" {
		b.WriteString(preserved)
	} else {
		b.WriteString(analysisMarker + "\n\n_Analysis pending — see the table above and out/*.png for a first look._\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
