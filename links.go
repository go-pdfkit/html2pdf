// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine"
)

// Navigation — the clickable runs an <a href> spans per line, the ids a
// fragment can jump to, the headings that become the bookmark tree — is a
// property of the laid-out document, not of PDF, and lives in the engine
// (engine.LinkRuns, engine.DocumentIDs, engine.Headings; #139). What
// stays here is the pagination side: which page a run or an id falls on,
// and clipping a box run to the page it starts on.

// addLinks attaches the clickable runs that start on the current page: a
// URI action for an external target, a GoTo to a named destination for an
// in-document one. A text run breaks per line, and pagination breaks between
// lines, so it is on exactly one page; a box run (an atom-less anchor) may
// straddle a break and is clipped to the page it starts on.
func (e *exporter) addLinks(links []engine.LinkRun) {
	for _, l := range links {
		if l.Y < e.pageTop || l.Y >= e.pageBot {
			continue
		}
		bottom := l.Y + l.H
		if bottom > e.pageBot {
			bottom = e.pageBot
		}
		x0, y0 := e.toPdf(l.X, l.Y)
		x1, y1 := e.toPdf(l.X+l.W, bottom)
		r := pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1}
		if l.URI != "" {
			e.p.AddLink(r, l.URI)
		} else {
			e.p.AddNamedLink(r, l.Fragment)
		}
	}
}

// addDests anchors, on the current page, every id whose element sits on it.
func (e *exporter) addDests(ids map[string]engine.Anchor) {
	for id, pt := range ids {
		if pt.Y < e.pageTop || pt.Y >= e.pageBot {
			continue
		}
		x, y := e.toPdf(pt.X, pt.Y)
		e.p.AddNamedDest(id, x, y)
	}
}

// pageIndexOf returns the 0-based page whose slice [tops[i], tops[i+1])
// holds y.
func pageIndexOf(y float64, tops []float64) int {
	idx := 0
	for i, t := range tops {
		if y >= t {
			idx = i
		}
	}
	return idx
}
