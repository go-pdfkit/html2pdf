// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"image"

	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
)

// exporter holds the state for painting one page's slice of the box tree.
type exporter struct {
	fonts    *fontSet
	imgs     map[*dom.Node]image.Image // decoded bitmaps, keyed by <img>/<svg> element
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
		e.p.SetLineWidth(side.Width * e.scale * pxToPt)
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

// paintLine draws one line box's items — text runs, and image items as
// embedded bitmaps — skipping the line entirely if it doesn't intersect the
// current page slice.
func (e *exporter) paintLine(line *layout.LineBox) {
	if line.Y+line.H <= e.pageTop || line.Y >= e.pageBot {
		return
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
func (e *exporter) paintImage(it *layout.InlineItem) {
	bmp, ok := e.imgs[it.Image]
	if !ok || bmp == nil || it.ImgW <= 0 || it.ImgH <= 0 {
		return
	}
	x0, y0 := e.toPdf(it.X, it.Y)
	x1, y1 := e.toPdf(it.X+it.ImgW, it.Y+it.ImgH)
	e.p.DrawImage(bmp, pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1})
}
