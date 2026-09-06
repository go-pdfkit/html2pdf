// Package mdreport writes a generated Markdown report while preserving the
// hand-written analysis a previous run left below a marker line — the
// convention engine-webengine/bench established for its REPORT.md, shared
// here by the corpus and bench harnesses so a re-run never clobbers notes.
package mdreport

import (
	"os"
	"strings"
)

// Marker separates the regenerated part of a report (above) from the
// preserved, hand-written analysis (below).
const Marker = "<!-- BEGIN ANALYSIS -->"

// Placeholder is written below the marker on a first run, when there is no
// analysis to preserve yet.
const Placeholder = Marker + "\n\n_Analysis pending — see the table above and out/ for a first look._\n"

// Write replaces path's content above Marker with generated (which must not
// itself contain the marker) and keeps everything from the marker on, or
// writes Placeholder there when the file did not exist or had no marker.
func Write(path, generated string) error {
	preserved := Placeholder
	if old, err := os.ReadFile(path); err == nil {
		if i := strings.Index(string(old), Marker); i >= 0 {
			preserved = string(old)[i:]
		}
	}
	return os.WriteFile(path, []byte(generated+preserved), 0o644)
}
