// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
)

// exporter holds the state for painting one page's slice of the box tree.
type exporter struct {
	fonts    *fontSet
	imgs     map[*dom.Node]*engine.LoadedImage // bitmaps and their sources, keyed by <img>/<svg> element
	imageDPI float64                           // Options.ImageDPI; 0 = keep the engine's pixels
	pageWPt  float64
	pageHPt  float64
	marginPt float64
	scale    float64 // viewport px -> print-column px (see Options.ViewportPx)
	pageTop  float64 // viewport px, top of the current page's content slice
	pageBot  float64 // viewport px
	p        *pdfkit.Page
}

// toPdf converts a viewport-space (px) point to this page's PDF point space,
// applying the print-column scale.
func (e *exporter) toPdf(xPx, yPx float64) (x, y float64) {
	x = e.marginPt + xPx*e.scale*pxToPt
	y = e.pageHPt - e.marginPt - (yPx-e.pageTop)*e.scale*pxToPt
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
	e.paintDecoration(b.Style, b.X, b.Y, b.W, b.H, true, true)
	for _, line := range b.Lines {
		e.paintLine(line)
	}
	for _, c := range b.Children {
		e.paintBox(c)
	}
}

// paintDecoration paints a border box's background then its borders, clipped
// to the current page's vertical slice — the one code path for a block-level
// Box and for an inline element's per-line fragment. left/right say whether
// this rectangle carries the element's left and right edges: always for a
// Box; for a fragment, only its First/Last one (box-decoration-break: slice),
// so a span wrapped over two lines is open where it continues.
//
// A box spanning a page break paints on every page it touches, but its top
// border only on the page where that edge actually falls (likewise bottom),
// so nothing draws a border across the middle of a page; left/right draw the
// full clipped slice height on each page.
func (e *exporter) paintDecoration(st *css.Style, x, y, w, h float64, left, right bool) {
	if st == nil || w <= 0 || h <= 0 {
		return
	}
	top, bot := y, y+h
	if top >= e.pageBot || bot <= e.pageTop {
		return
	}
	ct, cb := top, bot
	if ct < e.pageTop {
		ct = e.pageTop
	}
	if cb > e.pageBot {
		cb = e.pageBot
	}
	x0, y0 := e.toPdf(x, ct)
	x1, y1 := e.toPdf(x+w, cb)
	if st.Background.A > 0 {
		e.p.SetFillColor(toRGB(st.Background))
		e.p.Rectangle(pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1})
		e.p.Fill()
	}
	bw := st.Border.Widths()
	if bw.Top > 0 && top >= e.pageTop {
		e.strokeSide(x0, y0, x1, y0, st.Border.Top)
	}
	if bw.Bottom > 0 && bot <= e.pageBot {
		e.strokeSide(x0, y1, x1, y1, st.Border.Bottom)
	}
	if left && bw.Left > 0 {
		e.strokeSide(x0, y0, x0, y1, st.Border.Left)
	}
	if right && bw.Right > 0 {
		e.strokeSide(x1, y0, x1, y1, st.Border.Right)
	}
}

// strokeSide draws one border edge if it is present — non-zero width, a
// style other than none, a non-transparent colour — per css.Borders.Widths'
// own definition of "present" (mirrored here since that predicate is
// unexported).
func (e *exporter) strokeSide(x0, y0, x1, y1 float64, side css.BorderSide) {
	if side.Width <= 0 || side.Style == css.BorderNone || side.Color.A == 0 {
		return
	}
	e.p.SetStrokeColor(toRGB(side.Color))
	e.p.SetLineWidth(side.Width * e.scale * pxToPt)
	e.p.MoveTo(x0, y0)
	e.p.LineTo(x1, y1)
	e.p.Stroke()
}

// paintLine draws one line box: first the decoration of the inline elements
// crossing it — background then borders per fragment, outermost element
// first so a nested badge lands on top of its container's fill — then its
// items, text runs and image items as embedded bitmaps. A line that doesn't
// intersect the current page slice is skipped entirely, fragments included:
// a fragment's vertical padding may reach past the line box (CSS doesn't
// grow a line for it), and that overflow is clipped to this page rather than
// re-painted on the neighbouring one.
func (e *exporter) paintLine(line *layout.LineBox) {
	if line.Y+line.H <= e.pageTop || line.Y >= e.pageBot {
		return
	}
	for i := range line.Inlines {
		fr := &line.Inlines[i]
		e.paintDecoration(fr.Style, fr.X, fr.Y, fr.W, fr.H, fr.First, fr.Last)
	}
	for _, it := range line.Items {
		if it.Image != nil {
			e.paintImage(it)
			continue
		}
		if it.Text == "" || it.Style == nil {
			continue
		}
		f := e.fonts.pick(it.Style.FontFamily, it.Style.Bold(), it.Style.Italic)
		e.p.SetFont(f, it.Style.FontSize*e.scale*pxToPt)
		e.p.SetFillColor(toRGB(it.Style.Color))
		x, y := e.toPdf(it.X, it.Y+it.Ascent)
		_ = e.p.TextShaped(x, y, it.Text)
	}
}

// paintImage embeds an image item's decoded bitmap into the box layout gave
// it. The engine's raster painter blits the bitmap at (X, Y) at its native
// size, which its loader already made equal to (ImgW, ImgH) — so that box,
// scaled to the print column, is the destination rectangle. An element whose
// fetch or decode failed has no bitmap and draws nothing, same as on the
// raster canvas. Unlike a background, an image that straddles a page break is
// not clipped per page — it's an atom (its own line box), so pagination
// already keeps it whole; the page-slice test on the line is enough.
