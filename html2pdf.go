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
// background and borders paint per line fragment from the engine's own
// LineBox.Inlines (box-decoration-break: slice); border-radius,
// background-image and box-shadow on an inline element do not.
//
// Images — raster <img>, <img src="*.svg"> and inline <svg> — are fetched,
// decoded and sized by the engine's own pipeline (Engine.LoadImages) and
// embedded as bitmaps, so they are laid out and drawn exactly as the engine's
// raster canvas would. A relative src resolves against Options.BaseURL; an
// image that fails to fetch or decode is simply left out. This is the one
// place Export touches the network.
//
// # Navigation
//
// An <a href> becomes a link annotation — a URI action for an http(s)
// target, a GoTo to a named destination for a fragment that points at an
// element id in the document — one clickable rectangle per line the anchor
// spans, or the box of an anchor that lays out no text (see links.go). Every
// element id becomes a named destination, and
// the headings become the viewer's bookmark tree (<h1> at the top level,
// deeper headings nested under the last shallower one). The <title> fills
// the PDF's Title unless Options.Title is set.
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
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine"
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

	// BaseURL is the document's own URL, used to resolve a relative <img src>
	// and a relative <a href> (and to satisfy same-origin-shaped fetch logic
	// in the engine). A link whose href is this URL plus a fragment becomes an
	// in-document jump. Leave it empty for a document whose images and links
	// are all absolute or data: URIs.
	BaseURL string

	// Title, Author, Subject and Keywords fill the PDF's information
	// dictionary. An empty Title is taken from the document's <title>.
	Title, Author, Subject, Keywords string
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

	// Images go through the engine's own fetch/decode/size pipeline so their
	// boxes are laid out at the exact size the bitmaps come back at (an image
	// wider than the viewport is already scaled down there) — the same maps
	// RenderDocument feeds to its raster painter.
	imgDoc := &engine.Document{URL: opts.BaseURL, Root: root, HTML: htmlSrc}
	imgSizes, imgs := engine.New().LoadImages(context.Background(), imgDoc, sm, int(viewportPx))

	box, _ := layout.LayoutDocument(root, sm, viewportPx, fonts, imgSizes)

	fs, err := loadFonts()
	if err != nil {
		return nil, fmt.Errorf("html2pdf: load fonts: %w", err)
	}

	atoms := collectAtoms(box)
	breaks := pageBreaks(atoms, pageHViewportPx)
	tops := append([]float64{0}, breaks...)

	// Navigation: the anchors' clickable runs, the ids they can jump to, and
	// the headings that become the viewer's bookmark tree.
	base, _ := url.Parse(opts.BaseURL)
	ids := collectIDs(box)
	idSet := make(map[string]struct{}, len(ids))
	for id := range ids {
		idSet[id] = struct{}{}
	}
	links := collectLinks(box, base, idSet)
	heads := collectHeadings(box)

	title := opts.Title
	if title == "" {
		title = strings.Join(strings.Fields(dom.Title(root)), " ")
	}
	// Compress content and font streams: a text-heavy page's PDF is 6–16×
	// smaller for it (RFC 9110 29 → ~5 MB) and the output stays deterministic,
	// flate being deterministic. Image streams pick their own filter.
	doc := pdfkit.New(pdfkit.Options{Compress: true, Title: title, Author: opts.Author, Subject: opts.Subject, Keywords: opts.Keywords})
	e := &exporter{fonts: fs, imgs: imgs, pageWPt: pageWPt, pageHPt: pageHPt, marginPt: marginPt, scale: scale}
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
		e.addLinks(links)
		e.addDests(ids)
	}
	for _, h := range heads {
		doc.AddOutlineItem(h.title, h.level, pageIndexOf(h.y, tops))
	}
	return doc, nil
}

// addLinks attaches the clickable runs that start on the current page: a
// URI action for an external target, a GoTo to a named destination for an
// in-document one. A text run breaks per line, and pagination breaks between
// lines, so it is on exactly one page; a box run (an atom-less anchor) may
// straddle a break and is clipped to the page it starts on.
func (e *exporter) addLinks(links []linkRun) {
	for _, l := range links {
		if l.y < e.pageTop || l.y >= e.pageBot {
			continue
		}
		bottom := l.y + l.h
		if bottom > e.pageBot {
			bottom = e.pageBot
		}
		x0, y0 := e.toPdf(l.x, l.y)
		x1, y1 := e.toPdf(l.x+l.w, bottom)
		r := pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1}
		if l.uri != "" {
			e.p.AddLink(r, l.uri)
		} else {
			e.p.AddNamedLink(r, l.dest)
		}
	}
}

// addDests anchors, on the current page, every id whose element sits on it.
func (e *exporter) addDests(ids map[string]anchorPoint) {
	for id, pt := range ids {
		if pt.y < e.pageTop || pt.y >= e.pageBot {
			continue
		}
		x, y := e.toPdf(pt.x, pt.y)
		e.p.AddNamedDest(id, x, y)
	}
}
