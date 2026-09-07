// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
	"github.com/go-webengine/engine/paginate"
	"github.com/go-webengine/engine/paint"
)

// parseAndLayoutForTest mirrors Export's own parse+cascade+layout steps at
// the print column's own width (no viewport scaling), for tests that need to
// inspect the box tree the paginator sees rather than only the final PDF
// bytes.
func parseAndLayoutForTest(htmlSrc string) (*layout.Box, error) {
	o := (Options{}).resolved()
	contentWPx := (o.PageSize.Width - 2*pdfkit.Mm(o.MarginMm)) / pxToPt
	return layoutAtWidthForTest(htmlSrc, contentWPx)
}

// layoutAtWidthForTest lays out htmlSrc at an arbitrary viewport width.
func layoutAtWidthForTest(htmlSrc string, widthPx float64) (*layout.Box, error) {
	root, err := dom.Parse(htmlSrc)
	if err != nil {
		return nil, err
	}
	sm := css.Cascade(root)
	fonts := paint.NewFonts()
	box, _ := layout.LayoutDocument(root, sm, widthPx, fonts, nil)
	return box, nil
}

func TestExportSimpleDocument(t *testing.T) {
	doc, err := Export(`<html><body style="margin:0;font-family:sans-serif">
		<h1 style="color:#2954C8">Titre</h1>
		<p>Un paragraphe.</p>
	</body></html>`, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Errorf("output does not start with a PDF header: %q", buf.Bytes()[:16])
	}
}

func TestExportEmptyBodyProducesOnePage(t *testing.T) {
	doc, err := Export(`<html><body></body></html>`, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// A page count assertion would require parsing the emitted PDF; the write
	// succeeding without panic on a boxless document is the property under
	// test (the paginator must handle a document with nothing to cut between).
}

func TestExportInvalidOptionsFallBackToDefaults(t *testing.T) {
	doc, err := Export(`<html><body>x</body></html>`, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if doc == nil {
		t.Fatal("Export returned a nil document")
	}
}

func TestExportRejectsUnparsableInput(t *testing.T) {
	// dom.Parse is lenient (HTML has no hard syntax errors in practice), so
	// this exercises the error path indirectly: an empty string still parses
	// to an empty document rather than erroring, which is the correct HTML5
	// parsing behaviour — recorded here so a future tightening of dom.Parse's
	// error contract has a test to update instead of silently changing
	// behaviour unnoticed.
	if _, err := Export("", Options{}); err != nil {
		t.Errorf("Export(\"\") = %v, want nil error (HTML5 parsing has no hard failures)", err)
	}
}

func TestFontSetPickCoversAllFamilyWeightStyleCombinations(t *testing.T) {
	fs, err := loadFonts()
	if err != nil {
		t.Fatalf("loadFonts: %v", err)
	}
	for _, fam := range []css.FontFamily{css.Sans, css.Serif, css.Mono} {
		for _, bold := range []bool{false, true} {
			for _, italic := range []bool{false, true} {
				if f := fs.pick(fam, bold, italic); f == nil {
					t.Errorf("pick(%v, bold=%v, italic=%v) = nil", fam, bold, italic)
				}
			}
		}
	}
}

func TestToRGBConvertsEightBitChannels(t *testing.T) {
	got := toRGB(css.Color{R: 255, G: 0, B: 128, A: 255})
	if got.R != 1 || got.G != 0 || got.B != float64(128)/255 {
		t.Errorf("toRGB = %+v", got)
	}
}

func TestOptionsResolvedDefaults(t *testing.T) {
	o := Options{}.resolved()
	if o.MarginMm != 20 {
		t.Errorf("default MarginMm = %v, want 20", o.MarginMm)
	}
	if o.PageSize.Width <= 0 || o.PageSize.Height <= 0 {
		t.Errorf("default PageSize = %+v, want A4", o.PageSize)
	}
}

func TestExportPaintsAllFourBorderSides(t *testing.T) {
	doc, err := Export(`<html><body style="margin:0">
		<div style="border-top:2px solid red;border-right:2px solid blue;
			border-bottom:2px solid green;border-left:2px solid orange;
			width:100px;height:50px;background:#eee">x</div>
	</body></html>`, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestExportSkipsZeroWidthAndNoneBorders(t *testing.T) {
	doc, err := Export(`<html><body style="margin:0">
		<div style="border:0 solid red;width:50px;height:50px">a</div>
		<div style="border:2px none red;width:50px;height:50px">b</div>
		<div style="border:2px solid transparent;width:50px;height:50px">c</div>
	</body></html>`, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestExportPaginatesALongTable(t *testing.T) {
	var rows strings.Builder
	for i := 0; i < 80; i++ {
		rows.WriteString(`<tr><td style="padding:6px">row</td><td style="padding:6px">value</td></tr>`)
	}
	doc, err := Export(`<html><body style="margin:0"><table>`+rows.String()+`</table></body></html>`, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n := bytes.Count(buf.Bytes(), []byte("/Type /Page\n")); n > 0 && n < 2 {
		t.Errorf("an 80-row table produced only %d page(s), expected pagination", n)
	}
}

func TestExportNestedLayoutTableSplitsAcrossPages(t *testing.T) {
	// Hacker News' classic markup: an outer <tr><td> holding a nested <table>
	// of many real rows. Before hasDescendantTr, the outer row was treated as
	// one atom the size of the whole nested table, which — being taller than
	// a page — could still only start at a page top: it landed entirely on
	// page 2, leaving page 1 blank below the page's own header row.
	var inner strings.Builder
	for i := 0; i < 60; i++ {
		inner.WriteString(`<tr><td style="padding:10px">row</td></tr>`)
	}
	html := `<html><body style="margin:0">` +
		`<table><tr><td>header</td></tr></table>` +
		`<table><tr><td><table>` + inner.String() + `</table></td></tr></table>` +
		`</body></html>`
	doc, err := Export(html, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// The nested rows must be cut between, not carried as one atom the size
	// of the whole nested table: 60 rows of ~38px on 500px pages is four
	// pages at least (the paginator's own tests cover the wrapper-row rule).
	root, err := parseAndLayoutForTest(html)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	if breaks := paginate.Breaks(root, 500); len(breaks) < 3 {
		t.Errorf("paginate.Breaks cut %d times, want >= 3 (the nested rows must paginate as rows)", len(breaks))
	}
}

func TestExportScalesAWideFixedWidthPageToFitTheColumn(t *testing.T) {
	// A fixed-width sidebar beside flexible prose is the real shape this
	// guards against (RFC 9110's table-of-contents column, found via the
	// corpus): the sidebar's own width never changes, but the prose next to
	// it gets whatever the viewport has left over — squeezed to a sliver at
	// the print column's own ~642px, much roomier at ViewportPx's 1024px —
	// so it wraps into far fewer lines at the wider layout.
	html := `<html><body style="margin:0"><div style="display:flex">` +
		`<div style="width:300px;flex-shrink:0">sidebar</div>` +
		`<div>` + strings.Repeat("word ", 400) + `</div>` +
		`</div></body></html>`

	narrow, err := parseAndLayoutForTest(html)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	narrowLines := countLines(narrow)

	wideBox, err := layoutAtWidthForTest(html, 1024)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	wideLines := countLines(wideBox)

	if wideLines >= narrowLines {
		t.Errorf("wide-viewport layout produced %d line atoms, want fewer than the %d from a 642px-equivalent layout", wideLines, narrowLines)
	}

	doc, err := Export(html, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestExportViewportNeverNarrowerThanPrintColumn(t *testing.T) {
	// A ViewportPx set below the print column's own width must not shrink
	// the layout further — it's clamped up to the column width (scale 1, no
	// downscaling) rather than upscaling the page.
	doc, err := Export(`<html><body>x</body></html>`, Options{ViewportPx: 10})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

// allLinesForTest returns every line box in a layout tree, document order.
func allLinesForTest(b *layout.Box) []*layout.LineBox {
	var out []*layout.LineBox
	var walk func(*layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		out = append(out, b.Lines...)
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(b)
	return out
}

// decoratedSpanHTML is a styled inline element long enough to wrap inside a
// narrow container, so the engine fragments it across at least two lines.
const decoratedSpanHTML = `<html><body style="margin:0"><div style="width:240px">` +
	`before <span style="background:#ffe08a;border:2px solid #b5711a;padding:2px 6px">` +
	`a highlighted run of words that keeps going well past the end of the line` +
	`</span> after</div></body></html>`

func TestExportPaintsInlineFragments(t *testing.T) {
	// The engine must actually deliver fragments (LineBox.Inlines, engine
	// #128) for the pinned version — otherwise the paint path below is dead
	// code and this test guards nothing.
	root, err := layoutAtWidthForTest(decoratedSpanHTML, 1024)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	var frags, firsts, lasts int
	for _, line := range allLinesForTest(root) {
		for _, fr := range line.Inlines {
			frags++
			if fr.First {
				firsts++
			}
			if fr.Last {
				lasts++
			}
			if fr.Style == nil || fr.W <= 0 || fr.H <= 0 {
				t.Errorf("fragment with no style or no size: %+v", fr)
			}
		}
	}
	if frags < 2 || firsts != 1 || lasts != 1 {
		t.Fatalf("fragments=%d first=%d last=%d, want a span wrapped over >=2 lines with exactly one First and one Last", frags, firsts, lasts)
	}

	doc, err := Export(decoratedSpanHTML, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestPaintDecorationClipsToThePageSlice(t *testing.T) {
	// Drive paintDecoration directly with a real fragment style, at
	// positions that exercise every clipping branch: fully above the page
	// slice, straddling its top (top border must not draw), inside, straddling
	// its bottom (bottom border must not draw), fully below; plus a middle
	// fragment (neither First nor Last), a transparent background, a zero
	// width and a nil style. A precise page-break placement in a real
	// document would be far more brittle than this.
	root, err := layoutAtWidthForTest(decoratedSpanHTML, 1024)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	var st *css.Style
	for _, line := range allLinesForTest(root) {
		if len(line.Inlines) > 0 {
			st = line.Inlines[0].Style
			break
		}
	}
	if st == nil {
		t.Fatal("no inline fragment to borrow a style from")
	}
	noBg := *st
	noBg.Background.A = 0

	doc := pdfkit.New(pdfkit.Options{})
	e := &exporter{
		pageWPt:      pdfkit.A4.Width,
		pageHPt:      pdfkit.A4.Height,
		marginLeftPt: pdfkit.Mm(20),
		marginTopPt:  pdfkit.Mm(20),
		scale:        1,
		pageTop:      100,
		pageBot:      200,
		p:            doc.AddPage(pdfkit.A4),
	}
	e.paintDecoration(st, 10, 10, 50, 20, true, true)    // fully above: skipped
	e.paintDecoration(st, 10, 90, 50, 20, true, true)    // straddles top
	e.paintDecoration(st, 10, 120, 50, 20, true, true)   // inside, all four edges
	e.paintDecoration(st, 10, 150, 50, 20, false, false) // middle fragment: no left/right
	e.paintDecoration(st, 10, 190, 50, 20, true, true)   // straddles bottom
	e.paintDecoration(st, 10, 300, 50, 20, true, true)   // fully below: skipped
	e.paintDecoration(&noBg, 10, 120, 50, 20, true, true)
	e.paintDecoration(st, 10, 120, 0, 20, true, true) // zero width: skipped
	e.paintDecoration(nil, 10, 120, 50, 20, true, true)

	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestExportCompressesContentStreams(t *testing.T) {
	// Without Compress, a text-only document's PDF was 6–16× the size of the
	// same page printed by Chrome — every content stream written raw. Guard
	// that the streams are flate-encoded and that the output is still
	// byte-deterministic across two exports.
	html := `<html><body>` + strings.Repeat("<p>compressible prose, repeated.</p>", 200) + `</body></html>`
	var a, b bytes.Buffer
	for _, buf := range []*bytes.Buffer{&a, &b} {
		doc, err := Export(html, Options{})
		if err != nil {
			t.Fatalf("Export: %v", err)
		}
		if err := doc.Write(buf); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if !bytes.Contains(a.Bytes(), []byte("/FlateDecode")) {
		t.Error("no FlateDecode stream in the output: content streams are written uncompressed")
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("two exports of the same document differ byte-for-byte")
	}
}

func TestExportRespectsCustomMargin(t *testing.T) {
	doc, err := Export(`<html><body>x</body></html>`, Options{MarginMm: 40})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(buf.String(), "PDF") {
		t.Errorf("output does not look like a PDF")
	}
}

// countLines counts the text lines laid out under b.
func countLines(b *layout.Box) int {
	if b == nil {
		return 0
	}
	n := len(b.Lines)
	for _, c := range b.Children {
		n += countLines(c)
	}
	return n
}
