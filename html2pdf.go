// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package html2pdf renders static HTML straight to a vector PDF: it drives
// go-webengine's own layout tree (no screenshot, no raster slicing) into
// go-pdfkit text/rect/stroke calls. Pagination breaks between atoms — a text
// line, or a whole table row — never through one; see atoms.go.
//
// # Scope
//
// This is a static renderer: no JavaScript, no external stylesheets or
// @font-face (text uses the three families go-webengine's own paint package
// bundles — Inter for sans, Lora for serif, Go Mono for mono — so the glyphs
// drawn always match the metrics layout measured against). Inline-level
// background/border/padding do not paint: go-webengine's layout does not
// give a styled inline run its own box, only block/table/flex-level elements
// do (confirmed in the reference raster painter too — this is a shared engine
// limitation, not something this package works around). Inline `<svg>` and
// `<img>` are not yet painted; a document that needs a chart should draw it
// with plain block/table markup (backgrounds, borders, percentage widths)
// rather than inline SVG.
//
// # Quick start
//
//	doc, err := html2pdf.Export(htmlSource, html2pdf.Options{})
//	if err != nil { ... }
//	f, _ := os.Create("out.pdf")
//	defer f.Close()
//	doc.Write(f)
package html2pdf

import (
	"fmt"

	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
	"github.com/go-webengine/engine/paint"
)

// pxToPt converts a CSS px (1/96in, the unit go-webengine's layout works in)
// to a PDF point (1/72in).
const pxToPt = 72.0 / 96.0

// defaultViewportPx is the width a page is laid out against before being
// scaled down to fit the print column — see Options.ViewportPx.
const defaultViewportPx = 1024

// Options configures a single Export call. The zero value is A4, 20mm
// margins on all sides, and a 1024px layout viewport.
type Options struct {
	PageSize pdfkit.PageSize // zero value: pdfkit.A4
	MarginMm float64         // zero value: 20

	// ViewportPx is the width (CSS px) the page is laid out against, then
	// uniformly scaled down to fit the print column. Many real pages carry a
	// fixed-width element sized for a desktop viewport — a sidebar, a
	// multi-column nav — that a browser's own responsive CSS only collapses
	// below some breakpoint. Laying out directly at the print column's actual
	// width (a plain A4 page is under 650px wide) sits below most such
	// breakpoints, so that fixed-width element squeezes the rest of the page
	// into a narrow remainder and the whole document wraps far taller than it
	// needs to — confirmed against RFC 9110's HTML edition, whose
	// table-of-contents sidebar did exactly this (428 pages laid out at the
	// print column's own ~642px width vs. 184 at 1024px). Zero value: 1024,
	// a common small-desktop/tablet breakpoint. Set below the print column's
	// own width (rare) to lay out 1:1 with no scaling.
	ViewportPx float64
}

func (o Options) resolved() Options {
	if o.PageSize == (pdfkit.PageSize{}) {
		o.PageSize = pdfkit.A4
	}
	if o.MarginMm == 0 {
		o.MarginMm = 20
	}
	if o.ViewportPx == 0 {
		o.ViewportPx = defaultViewportPx
	}
	return o
}

// Export parses htmlSrc, lays it out at opts.ViewportPx and returns a
// paginated pdfkit.Document — scaled to fit the page's printable width —
// ready to Write.
func Export(htmlSrc string, opts Options) (*pdfkit.Document, error) {
	opts = opts.resolved()

	root, err := dom.Parse(htmlSrc)
	if err != nil {
		return nil, fmt.Errorf("html2pdf: parse: %w", err)
	}
	sm := css.Cascade(root)
	fonts := paint.NewFonts()

	pageWPt := opts.PageSize.Width
	pageHPt := opts.PageSize.Height
	marginPt := pdfkit.Mm(opts.MarginMm)
	contentWPt := pageWPt - 2*marginPt
	contentHPt := pageHPt - 2*marginPt
	contentWPx := contentWPt / pxToPt
	contentHPx := contentHPt / pxToPt

	viewportPx := opts.ViewportPx
	if viewportPx < contentWPx {
		viewportPx = contentWPx // never upscale — 1:1 is the narrowest layout
	}
	scale := contentWPx / viewportPx
	pageHViewportPx := contentHPx / scale // page-height budget in viewport space

	box, _ := layout.LayoutDocument(root, sm, viewportPx, fonts, nil)

	fs, err := loadFonts()
	if err != nil {
		return nil, fmt.Errorf("html2pdf: load fonts: %w", err)
	}

	atoms := collectAtoms(box)
	breaks := pageBreaks(atoms, pageHViewportPx)
	tops := append([]float64{0}, breaks...)

	doc := pdfkit.New(pdfkit.Options{})
	e := &exporter{fonts: fs, pageWPt: pageWPt, pageHPt: pageHPt, marginPt: marginPt, scale: scale}
	for i, top := range tops {
		bot := pageHViewportPx * 1e9 // effectively unbounded: the last page
		if i+1 < len(tops) {
			bot = tops[i+1]
		}
		e.pageTop, e.pageBot = top, top+pageHViewportPx
		if bot < e.pageBot {
			e.pageBot = bot
		}
		e.p = doc.AddPage(opts.PageSize)
		e.paintBox(box)
	}
	return doc, nil
}
