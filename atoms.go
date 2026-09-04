// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"sort"

	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
)

// atom is one indivisible vertical slice of content for pagination purposes:
// a single text line, or a whole table row (never split mid-row). Coordinates
// are in the layout viewport's px space (see Options.ViewportPx), before the
// print-column scale is applied.
type atom struct{ top, bottom float64 }

// collectAtoms walks the box tree and returns every atom in document order,
// sorted by top.
//
// A <tr> row is one atom regardless of how many lines its cells wrap to —
// splitting a row across pages reads worse than a few extra blank
// millimetres at the bottom of a page — unless that row is itself a
// layout-table wrapper (its cell holds a nested <table>, e.g. Hacker News'
// classic markup): swallowing that whole nested table into one atom made it
// taller than a page, so it could only start at a page top, wasting
// everything before it. hasDescendantTr tells the two cases apart; only a
// childless <tr> counts as one atom, a wrapper is descended into so its real
// rows become the atoms instead.
//
// Any other box's own text lines are each their own atom, so a paragraph can
// still break between lines. A childless, line-less box with real height (a
// rule, a spacer) gets one atom too, so its height is accounted for even
// though nothing inside it can break.
func collectAtoms(b *layout.Box) []atom {
	var out []atom
	var walk func(b *layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		if isRow(b) && !hasDescendantTr(b) {
			out = append(out, atom{b.Y, b.Y + b.H})
			return
		}
		for _, ln := range b.Lines {
			out = append(out, atom{ln.Y, ln.Y + ln.H})
		}
		if len(b.Children) == 0 && len(b.Lines) == 0 && b.H > 0 {
			out = append(out, atom{b.Y, b.Y + b.H})
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(b)
	sort.Slice(out, func(i, j int) bool { return out[i].top < out[j].top })
	return out
}

// isRow reports whether b is a <tr> box.
func isRow(b *layout.Box) bool {
	return b.Node != nil && b.Node.Type == dom.Element && b.Node.Tag == "tr"
}

// hasDescendantTr reports whether b's subtree contains another <tr> — the
// signature of a layout-table trick (a row whose cell holds a nested table)
// rather than a plain data row.
func hasDescendantTr(b *layout.Box) bool {
	for _, c := range b.Children {
		if isRow(c) || hasDescendantTr(c) {
			return true
		}
	}
	return false
}

// pageBreaks returns the y (viewport px) at which each page after the first
// starts, given the usable content height per page (viewport px). It only
// ever cuts between atoms — before whichever atom would otherwise overflow
// the page — so no line or table row is split across pages. An atom taller
// than pageH still gets exactly one break before it: it cannot be split
// further, so it simply overflows its own page's bottom margin rather than
// looping forever trying to fit it.
func pageBreaks(atoms []atom, pageH float64) []float64 {
	if len(atoms) == 0 {
		return nil
	}
	var breaks []float64
	pageTop := 0.0
	for _, a := range atoms {
		if a.bottom-pageTop > pageH && a.top > pageTop {
			breaks = append(breaks, a.top)
			pageTop = a.top
		}
	}
	return breaks
}
