// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package html2pdf renders static HTML straight to a vector PDF: it drives
// go-webengine's own layout tree (no screenshot, no raster slicing) into
// go-pdfkit text/rect/stroke calls. Pagination breaks between text lines and
// table rows, never through one.
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
	"sort"

	"github.com/go-opentype/fonts/gomono"
	"github.com/go-opentype/fonts/inter"
	"github.com/go-opentype/fonts/lora"
	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
	"github.com/go-webengine/engine/paint"
)

// pxToPt converts a CSS px (1/96in, the unit go-webengine's layout works in)
// to a PDF point (1/72in).
const pxToPt = 72.0 / 96.0

// Options configures a single Export call. The zero value is A4 with 20mm
// margins on all sides.
type Options struct {
	PageSize pdfkit.PageSize // zero value: pdfkit.A4
	MarginMm float64         // zero value: 20
}

func (o Options) resolved() Options {
	if o.PageSize == (pdfkit.PageSize{}) {
		o.PageSize = pdfkit.A4
	}
	if o.MarginMm == 0 {
		o.MarginMm = 20
	}
	return o
}

// Export parses htmlSrc, lays it out at the page's printable width and
// returns a paginated pdfkit.Document ready to Write. baseURL is unused today
// (no external resource fetching yet) and accepted for forward compatibility.
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

	box, _ := layout.LayoutDocument(root, sm, contentWPx, fonts, nil)

	fs, err := loadFonts()
	if err != nil {
		return nil, fmt.Errorf("html2pdf: load fonts: %w", err)
	}

	atoms := collectAtoms(box)
	breaks := pageBreaks(atoms, contentHPx)
	tops := append([]float64{0}, breaks...)

	doc := pdfkit.New(pdfkit.Options{})
	e := &exporter{fonts: fs, pageWPt: pageWPt, pageHPt: pageHPt, marginPt: marginPt}
	for i, top := range tops {
		bot := contentHPx * 1e9 // effectively unbounded: the last page
		if i+1 < len(tops) {
			bot = tops[i+1]
		}
		e.pageTop, e.pageBot = top, top+contentHPx
		if bot < e.pageBot {
			e.pageBot = bot
		}
		e.p = doc.AddPage(opts.PageSize)
		e.paintBox(box)
	}
	return doc, nil
}

// fontSet resolves the (family, bold, italic) requests the layout produced to
// loaded pdfkit fonts. Mono has only a regular face, matching paint.Fonts'
// own fallback (see engine's paint/fonts.go): the family ships no bold or
// italic style, so both requests render in the upright regular face.
type fontSet struct {
	sans, sansB, sansI, sansBI     *pdfkit.Font
	serif, serifB, serifI, serifBI *pdfkit.Font
	mono                           *pdfkit.Font
}

func loadFonts() (*fontSet, error) {
	fs := &fontSet{}
	for _, pair := range []struct {
		dst **pdfkit.Font
		b   []byte
	}{
		{&fs.sans, inter.TTF}, {&fs.sansB, inter.BoldTTF}, {&fs.sansI, inter.ItalicTTF}, {&fs.sansBI, inter.BoldItalicTTF},
		{&fs.serif, lora.TTF}, {&fs.serifB, lora.BoldTTF}, {&fs.serifI, lora.ItalicTTF}, {&fs.serifBI, lora.BoldItalicTTF},
		{&fs.mono, gomono.TTF},
	} {
		f, err := pdfkit.LoadFont(pair.b)
		if err != nil {
			return nil, err
		}
		*pair.dst = f
	}
	return fs, nil
}

func (fs *fontSet) pick(fam css.FontFamily, bold, italic bool) *pdfkit.Font {
	switch fam {
	case css.Serif:
		switch {
		case bold && italic:
			return fs.serifBI
		case bold:
			return fs.serifB
		case italic:
			return fs.serifI
		default:
			return fs.serif
		}
	case css.Mono:
		return fs.mono
	default:
		switch {
		case bold && italic:
			return fs.sansBI
		case bold:
			return fs.sansB
		case italic:
			return fs.sansI
		default:
			return fs.sans
		}
	}
}

// hasDescendantTr reports whether b's subtree contains another <tr> — the
// signature of a layout-table trick (a row whose cell holds a nested table)
// rather than a plain data row.
func hasDescendantTr(b *layout.Box) bool {
	for _, c := range b.Children {
		if c.Node != nil && c.Node.Type == dom.Element && c.Node.Tag == "tr" {
			return true
		}
		if hasDescendantTr(c) {
			return true
		}
	}
	return false
}

// atom is one indivisible vertical slice of content for pagination purposes:
// a single text line, or a whole table row (never split mid-row).
type atom struct{ top, bottom float64 }

// collectAtoms walks the box tree and returns every atom in document order,
// sorted by top. A <tr> row is one atom regardless of how many lines its
// cells wrap to — splitting a row across pages reads worse than a few extra
// blank millimetres at the bottom of a page. Any other box's own text lines
// are each their own atom, so a paragraph can still break between lines. A
// childless, line-less box with real height (a rule, a spacer) gets one atom
// too, so its height is accounted for even though nothing inside it can break.
func collectAtoms(b *layout.Box) []atom {
	var out []atom
	var walk func(b *layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		// A layout-table trick (a <tr> whose cell holds a nested <table>, e.g.
		// Hacker News' classic markup) must not become one giant atom: that
		// swallowed the inner rows entirely, so the whole nested table — often
		// many pages tall — could only start at a page top, wasting the rest
		// of whichever page it didn't fit. Only a <tr> with no <tr> inside it
		// is real tabular content and worth keeping whole.
		if b.Node != nil && b.Node.Type == dom.Element && b.Node.Tag == "tr" && !hasDescendantTr(b) {
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

// pageBreaks returns the y (px, document space) at which each page after the
// first starts, given the usable content height per page (px). It only ever
// cuts between atoms — before whichever atom would otherwise overflow the
// page — so no line or table row is split across pages.
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

// exporter holds the state for painting one page's slice of the box tree.
type exporter struct {
	fonts    *fontSet
	pageWPt  float64
	pageHPt  float64
	marginPt float64
	pageTop  float64 // px, top of the current page's content slice
	pageBot  float64 // px
	p        *pdfkit.Page
}

// toPdf converts a document-space (px) point to this page's PDF point space.
func (e *exporter) toPdf(xPx, yPx float64) (x, y float64) {
	x = e.marginPt + xPx*pxToPt
	y = e.pageHPt - e.marginPt - (yPx-e.pageTop)*pxToPt
	return
}

func toRGB(c css.Color) pdfkit.RGB {
	return pdfkit.RGB{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255}
}

// paintBox paints one box's background/border (clipped to the current page's
// vertical slice) and its text lines, then recurses into its children.
func (e *exporter) paintBox(b *layout.Box) {
	if b == nil {
		return
	}
	top, bot := b.Y, b.Y+b.H
	visible := top < e.pageBot && bot > e.pageTop
	if visible && b.Style != nil && b.W > 0 && b.H > 0 {
		ct, cb := top, bot
		if ct < e.pageTop {
			ct = e.pageTop
		}
		if cb > e.pageBot {
			cb = e.pageBot
		}
		if cb > ct {
			x0, y0 := e.toPdf(b.X, ct)
			x1, y1 := e.toPdf(b.X+b.W, cb)
			r := pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1}
			if b.Style.Background.A > 0 {
				e.p.SetFillColor(toRGB(b.Style.Background))
				e.p.Rectangle(r)
				e.p.Fill()
			}
			e.paintBorders(b, top, bot, x0, y0, x1, y1)
		}
	}
	for _, line := range b.Lines {
		e.paintLine(line)
	}
	for _, c := range b.Children {
		e.paintBox(c)
	}
}

// paintBorders draws each side present with non-zero width/style/alpha, per
// css.Borders.Widths' own definition of "present" (mirrored here since that
// predicate is unexported). Top/bottom only draw on the page where that edge
// actually falls, so a box spanning a page break doesn't paint a border
// across the middle of a page; left/right draw the full clipped slice height
// on every page the box appears on.
func (e *exporter) paintBorders(b *layout.Box, top, bot, x0, y0, x1, y1 float64) {
	bw := b.Style.Border.Widths()
	draw := func(x0, y0, x1, y1 float64, side css.BorderSide) {
		if side.Width <= 0 || side.Style == css.BorderNone || side.Color.A == 0 {
			return
		}
		e.p.SetStrokeColor(toRGB(side.Color))
		e.p.SetLineWidth(side.Width * pxToPt)
		e.p.MoveTo(x0, y0)
		e.p.LineTo(x1, y1)
		e.p.Stroke()
	}
	if bw.Top > 0 && top >= e.pageTop && top < e.pageBot {
		draw(x0, y0, x1, y0, b.Style.Border.Top)
	}
	if bw.Bottom > 0 && bot > e.pageTop && bot <= e.pageBot {
		draw(x0, y1, x1, y1, b.Style.Border.Bottom)
	}
	if bw.Left > 0 {
		draw(x0, y0, x0, y1, b.Style.Border.Left)
	}
	if bw.Right > 0 {
		draw(x1, y0, x1, y1, b.Style.Border.Right)
	}
}

// paintLine draws one line box's text items, skipping the line entirely if it
// doesn't intersect the current page slice.
func (e *exporter) paintLine(line *layout.LineBox) {
	if line.Y+line.H <= e.pageTop || line.Y >= e.pageBot {
		return
	}
	for _, it := range line.Items {
		if it.Text == "" || it.Style == nil {
			continue
		}
		f := e.fonts.pick(it.Style.FontFamily, it.Style.Bold(), it.Style.Italic)
		e.p.SetFont(f, it.Style.FontSize*pxToPt)
		e.p.SetFillColor(toRGB(it.Style.Color))
		x, y := e.toPdf(it.X, it.Y+it.Ascent)
		_ = e.p.TextShaped(x, y, it.Text)
	}
}
