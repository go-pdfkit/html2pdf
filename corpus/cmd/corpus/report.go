package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-pdfkit/html2pdf/corpus/internal/mdreport"
)

// writeReport renders results as a Markdown table above mdreport.Marker,
// preserving any hand-written analysis below it from a previous run.
func writeReport(path string, results []result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# html2pdf corpus run — %s\n\n", time.Now().UTC().Format("2006-01-02"))

	ok := 0
	for _, r := range results {
		if r.OK {
			ok++
		}
	}
	fmt.Fprintf(&b, "%d/%d succeeded.\n\n", ok, len(results))

	b.WriteString("| URL | Status | Pages | PDF | Text chars | Links | Fetch | Render |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, r := range results {
		status := "✅"
		if !r.OK {
			status = "❌ " + r.Error
		}
		fmt.Fprintf(&b, "| [%s](%s) | %s | %d | %d B | %d | %d | %dms | %dms |\n",
			r.URL, r.URL, status, r.Pages, r.PDFBytes, r.TextChars, r.Links, r.FetchMs, r.RenderMs)
	}
	b.WriteString("\n")
	return mdreport.Write(path, b.String())
}
