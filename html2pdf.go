// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package html2pdf renders static HTML straight to a vector PDF: it drives
// go-webengine's own layout tree (no screenshot, no raster slicing) into
// go-pdfkit text/rect/stroke calls. Pagination is the engine's
// (paginate.Breaks): it cuts between atoms — a text line, a whole table
// row — never through one, and honours the document's CSS fragmentation
// (break-before/after: page, break-inside: avoid, break-after: avoid,
// orphans, widows) and its @page size and margins.
//
// # Scope
//
// This is a static renderer: no JavaScript and no @font-face (text uses the
// three families go-webengine's own paint package bundles — Inter for sans,
// Lora for serif, Go Mono for mono — so the glyphs drawn always match the
// metrics layout measured against). External stylesheets — <link
// rel="stylesheet"> and their @import chains — are fetched through the
// engine's own bounded loader (Engine.LoadStylesheets) and cascaded for the
// print medium by default (Options.Media), so a page's @media print rules
// apply and its screen-only ones do not, as in a browser's print preview.
// Inline-level
// background and borders paint per line fragment from the engine's own
// LineBox.Inlines (box-decoration-break: slice); border-radius,
// background-image and box-shadow on an inline element do not.
//
// Images — raster <img>, <img src="*.svg"> and inline <svg> — are fetched,
// decoded and sized by the engine's own pipeline (Engine.LoadImageSet) and
// embedded so they are laid out and drawn exactly as the engine's raster
// canvas would: a JPEG source as its own bytes (DCTDecode), any other lossy
// source re-encoded as JPEG when opaque, everything else as a flate bitmap
// with a soft mask for transparency — see images.go and Options.ImageDPI. A
// relative src resolves against Options.BaseURL; an image that fails to
// fetch or decode is simply left out. Stylesheets and images are the two
// places Export touches the network.
//
// # Navigation
//
// An <a href> becomes a link annotation — a URI action for an http(s)
// target, a GoTo to a named destination for a fragment that points at an
// element id in the document — one clickable rectangle per line the anchor
// spans, or the box of an anchor that lays out no text (engine.LinkRuns). Every
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
	"math"
	"net/url"
	"strings"

	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
	"github.com/go-webengine/engine/paginate"
	"github.com/go-webengine/engine/paint"
)

// pxToPt converts a CSS px (1/96in, the unit go-webengine's layout works in)
// to a PDF point (1/72in).
const pxToPt = 72.0 / 96.0

// defaultViewportPx is the width a page is laid out against before being
// scaled down to fit the print column — see Options.ViewportPx.
const defaultViewportPx = 1024

// Options configures a single Export call. The zero value is A4, 20mm
// margins on all sides, and a 1024px layout viewport — unless the document
// says otherwise: an @page rule's size and margins win over PageSize and
// MarginMm, as they do in a browser's print (Chrome honours @page size in
// headless print-to-PDF; WeasyPrint always has).
type Options struct {
	PageSize pdfkit.PageSize // zero value: pdfkit.A4; overridden by @page { size }
	MarginMm float64         // zero value: 20; overridden by @page { margin }

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
	// a common small-desktop/tablet breakpoint — unless the document carries
	// an @page rule: a document that names its paper was designed for it,
	// its absolute units (a 12pt body, a 40mm figure) must print at their
	// true size, and it is laid out 1:1 as a browser prints it. Set below
	// the print column's own width (rare) to lay out 1:1 with no scaling.
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

	// ImageDPI caps the pixel density of an embedded bitmap at its painted
	// size: a bitmap that would exceed it — a 1024 px photograph painted
	// 60 mm wide is 430 dpi — is downsampled to it. Zero (the default) keeps
	// every pixel the engine fetched, which is what Chrome's print and
	// WeasyPrint do by default; WeasyPrint's --dpi is the same lever. 150
	// is a sound print value, 96 the screen's.
	ImageDPI float64

	// Media is the CSS medium the page is styled for: "print" (the zero
	// value) applies the page's @media print rules and print-only
	// stylesheets and skips its screen-only ones — a browser's print
	// preview, where a site's navigation, sidebars and footers are usually
	// hidden; "screen" styles the page as displayed. Width features
	// (min-width, max-width) are evaluated at ViewportPx under either.
	Media string
}

func (o Options) resolved() Options {
	if o.PageSize == (pdfkit.PageSize{}) {
		o.PageSize = pdfkit.A4
	}
	if o.MarginMm == 0 {
		o.MarginMm = 20
	}
	if o.ViewportPx == 0 {
		o.ViewportPx = -defaultViewportPx // the default, marked as such (see Export)
	}
	if o.Media == "" {
		o.Media = css.Print
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
	fonts := paint.NewFonts()

	// Stylesheets go through the engine's own bounded <link>/@import fetch
	// with media selection, so the page is styled exactly as the engine
	// styles it; they are fetched first because the document's @page rule
	// — size and margins — may live in one of them and decides the geometry
	// everything below is laid out against.
	ctx := context.Background()
	eng := engine.New()
	webDoc := &engine.Document{URL: opts.BaseURL, Root: root, HTML: htmlSrc}
	media := css.Media{Type: opts.Media, Width: math.Abs(opts.ViewportPx)}
	sheets := eng.LoadStylesheets(ctx, webDoc, media)

	page := css.DocumentPage(root, sheets, media, "")
	pageWPt, pageHPt, margins := pageGeometry(opts, page)
	contentWPt := pageWPt - margins[3] - margins[1]
	contentHPt := pageHPt - margins[0] - margins[2]
	contentWPx := contentWPt / pxToPt
	contentHPx := contentHPt / pxToPt

	// The layout width: the caller's, or the default — which yields to 1:1
	// for a document that declared its paper (see Options.ViewportPx).
	viewportPx := opts.ViewportPx
	if viewportPx < 0 {
		viewportPx = -viewportPx
		if page.Width > 0 || page.MarginSet != [4]bool{} {
			viewportPx = contentWPx
		}
	}
	if viewportPx < contentWPx {
		viewportPx = contentWPx // never upscale — 1:1 is the narrowest layout
	}
	scale := contentWPx / viewportPx
	pageHViewportPx := contentHPx / scale // page-height budget in viewport space

	// Images go through the engine's fetch/decode/size pipeline too, so
	// image boxes are laid out at the exact size the bitmaps come back at
	// (an image wider than the viewport is already scaled down there): the
	// same sheets and maps RenderDocument feeds its own cascade and raster
	// painter, cascaded here for opts.Media at the layout width.
	media.Width = viewportPx
	sm := css.CascadeMedia(root, media, sheets)
	imgs := eng.LoadImageSet(ctx, webDoc, sm, int(viewportPx))
	imgSizes := make(map[*dom.Node][2]float64, len(imgs))
	for n, li := range imgs {
		imgSizes[n] = li.Size
	}

	box, _ := layout.LayoutDocument(root, sm, viewportPx, fonts, imgSizes)

	fs, err := loadFonts()
	if err != nil {
		return nil, fmt.Errorf("html2pdf: load fonts: %w", err)
	}

	breaks := paginate.Breaks(box, pageHViewportPx)
	tops := append([]float64{0}, breaks...)

	// Navigation: the anchors' clickable runs, the ids they can jump to, and
	// the headings that become the viewer's bookmark tree.
	base, _ := url.Parse(opts.BaseURL)
	ids := engine.DocumentIDs(box)
	idSet := make(map[string]struct{}, len(ids))
	for id := range ids {
		idSet[id] = struct{}{}
	}
	links := engine.LinkRuns(box, base, idSet)
	heads := engine.Headings(box)

	title := opts.Title
	if title == "" {
		title = strings.Join(strings.Fields(dom.Title(root)), " ")
	}
	// Compress content and font streams: a text-heavy page's PDF is 6–16×
	// smaller for it (RFC 9110 29 → ~5 MB) and the output stays deterministic,
	// flate being deterministic. Image streams pick their own filter. Object
	// streams (PDF 1.5) pack the thousands of small non-stream objects a
	// linked document carries — one annotation per clickable line — into
	// flated streams: ~14 B per link instead of ~200 (pdfkit #29).
	doc := pdfkit.New(pdfkit.Options{Compress: true, ObjectStreams: true, Title: title, Author: opts.Author, Subject: opts.Subject, Keywords: opts.Keywords})
	pageSize := pdfkit.PageSize{Width: pageWPt, Height: pageHPt}
	e := &exporter{fonts: fs, imgs: imgs, imageDPI: opts.ImageDPI, pageWPt: pageWPt, pageHPt: pageHPt, marginLeftPt: margins[3], marginTopPt: margins[0], scale: scale}
	for i, top := range tops {
		bot := pageHViewportPx * 1e9 // effectively unbounded: the last page
		if i+1 < len(tops) {
			bot = tops[i+1]
		}
		e.pageTop, e.pageBot = top, top+pageHViewportPx
		if bot < e.pageBot {
			e.pageBot = bot
		}
		e.p = doc.AddPage(pageSize)
		e.paintBox(box)
		e.addLinks(links)
		e.addDests(ids)
	}
	for _, h := range heads {
		doc.AddOutlineItem(h.Title, h.Level, pageIndexOf(h.Y, tops))
	}
	return doc, nil
}

// pageGeometry resolves the page box: Options' size and uniform margin,
// overridden by whatever the document's @page rule set — a size when it
// gave one, each margin side it named. Everything in PDF points; margins
// are top, right, bottom, left.
func pageGeometry(opts Options, page css.PageSpec) (wPt, hPt float64, margins [4]float64) {
	wPt, hPt = opts.PageSize.Width, opts.PageSize.Height
	if page.Width > 0 && page.Height > 0 {
		wPt, hPt = page.Width*pxToPt, page.Height*pxToPt
	}
	for i := range margins {
		margins[i] = pdfkit.Mm(opts.MarginMm)
		if page.MarginSet[i] {
			margins[i] = page.Margin[i] * pxToPt
		}
	}
	return wPt, hPt, margins
}
