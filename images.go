// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"

	"github.com/go-gfx/gfx/codec"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/resample"
	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/layout"
)

// jpegQuality is the quality a lossy source is re-encoded at when its bytes
// cannot be passed through. Chrome's print-to-PDF re-encodes opaque bitmaps
// at what its quantisation tables put at about 50 (measured on its output
// with Pillow); 85 keeps the photographs visibly clean and still lands at a
// tenth of the flate bitmap's size.
const jpegQuality = 85

// paintImage draws one replaced element, choosing how the pixels are
// stored by what the engine fetched (see Options.ImageDPI for the sizing):
//
//   - a JPEG source a PDF reader decodes directly is embedded as it is — a
//     DCTDecode stream of the original bytes, the rule Skia (Chrome's PDF
//     backend) applies; only YCbCr and grey JPEGs qualify, a CMYK JPEG
//     would need an inverted Decode array Adobe writers expect;
//   - any other lossy source (a WebP, a JPEG that had to be resampled for
//     an ImageDPI cap) that is opaque is re-encoded as JPEG at
//     jpegQuality — lossy again, as Chrome does, rather than stored
//     losslessly at ten times the size;
//   - everything else — PNG, GIF, SVG rasters, anything with transparency —
//     is a flate bitmap with a soft mask when it has alpha, so line art and
//     screenshots keep every pixel.
//
// The pixels come from the source rather than from the engine's bitmap
// wherever the source bytes are still around — see sourcePixels.
func (e *exporter) paintImage(it *layout.InlineItem) {
	li, ok := e.imgs[it.Image]
	if !ok || li == nil || li.Bitmap == nil || it.ImgW <= 0 || it.ImgH <= 0 {
		return
	}
	// The box LAYOUT gives the element, not the bitmap's own size. ImgW/ImgH
	// are the loaded bitmap's pixels — paint's resample SOURCE — while Width
	// and LineHeight carry the display size, after the element's own width and
	// max-width are resolved against its real containing width (engine
	// layout.resolvedReplacedSize). The two coincide only for an image no CSS
	// resized, so drawing at ImgW/ImgH put every `max-width: 100%` image on
	// the page at its source size: on the A0 poster the photographs landed
	// 3179 px wide inside 912 px columns.
	dw, dh := it.Width, it.LineHeight
	if dw <= 0 || dh <= 0 {
		dw, dh = it.ImgW, it.ImgH
	}
	x0, y0 := e.toPdf(it.X, it.Y)
	x1, y1 := e.toPdf(it.X+dw, it.Y+dh)
	r := pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1}

	// The source's own bytes, byte for byte, whenever a PDF reader decodes
	// them and no cap asks for fewer pixels than they carry — whether or
	// not the engine kept that many for its own canvas.
	if li.Format == "jpeg" && len(li.Data) > 0 && !overDPI(li.SourceW, li.SourceH, r, e.imageDPI) && jpegPassable(li.Data) {
		if e.p.DrawJPEG(li.Data, r) == nil {
			return
		}
	}
	// An inline <svg> or an <img src="*.svg"> is a DRAWING, not pixels: the
	// engine rasterised it once at its CSS box, which is 96 dpi on paper.
	// Render it again at the density this page is being made at.
	if bmp, ok := svgAtDensity(li, r, e.imageDPI); ok {
		e.p.DrawImage(bmp, r)
		return
	}
	bmp := sourcePixels(li)
	if e.imageDPI > 0 {
		bmp = downsampleFor(bmp, r, e.imageDPI)
	}
	if li.Lossy && isOpaque(bmp) {
		var buf bytes.Buffer
		if jpeg.Encode(&buf, bmp, &jpeg.Options{Quality: jpegQuality}) == nil && e.p.DrawJPEG(buf.Bytes(), r) == nil {
			return
		}
	}
	e.p.DrawImage(bmp, r)
}

// sourcePixels returns the pixels to embed for one image: the source's own
// wherever its bytes are still around, else the engine's bitmap.
//
// The engine decodes and sizes an image for its raster canvas, where one
// CSS px is one device px — so LoadedImage.Bitmap carries the element's CSS
// box worth of pixels and no more. On paper that is a ceiling of exactly 96
// dpi for a document laid out 1:1 against its own @page (the A0 poster this
// was found on: every one of its 51 images landed at 96 dpi, and its 1800 px
// photograph had been resampled UP to 3179 to fill the viewport, so the file
// was larger than the source and carried less of it). A PDF has no such
// constraint — the printer's RIP resamples at whatever density the paper
// deserves — so where Data is still there (every <img>; an inline <svg> has
// none) the source is decoded again at full size and embedded instead. This
// is what Chrome's print does: its own output of that poster carries images
// from 121 to 739 dpi. Options.ImageDPI caps the result.
func sourcePixels(li *engine.LoadedImage) image.Image {
	if bitmapIsSource(li) || len(li.Data) == 0 || li.Format == "svg" || li.SourceW <= 0 || li.SourceH <= 0 {
		return li.Bitmap
	}
	img, err := codec.Decode(li.Data)
	if err != nil || img == nil || img.W != li.SourceW || img.H != li.SourceH {
		return li.Bitmap // undecodable a second time, or not the image the engine measured
	}
	return img.ToNRGBA()
}

// overDPI reports whether a w×h source carries more pixels than dpi allows
// over the rectangle it is painted into — the test that decides whether the
// source bytes can be passed through untouched. A zero cap never does.
func overDPI(w, h int, r pdfkit.Rect, dpi float64) bool {
	if dpi <= 0 {
		return false
	}
	return float64(w) > math.Ceil(r.Width/72*dpi) || float64(h) > math.Ceil(r.Height/72*dpi)
}

// downsampleFor scales bmp down so that it carries no more than dpi pixels
// per inch of the rectangle it is painted into; a bitmap already at or
// below that density is returned as it is. go-gfx's bicubic resampler —
// the one the engine itself sizes images with — deterministic.
func downsampleFor(bmp image.Image, r pdfkit.Rect, dpi float64) image.Image {
	b := bmp.Bounds()
	maxW := int(math.Ceil(r.Width / 72 * dpi))
	maxH := int(math.Ceil(r.Height / 72 * dpi))
	if maxW < 1 || maxH < 1 || (b.Dx() <= maxW && b.Dy() <= maxH) {
		return bmp
	}
	sx, sy := float64(maxW)/float64(b.Dx()), float64(maxH)/float64(b.Dy())
	s := math.Min(sx, sy)
	w, h := int(math.Round(float64(b.Dx())*s)), int(math.Round(float64(b.Dy())*s))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	out, err := resample.Resize(raster.FromImage(bmp), w, h, resample.Bicubic)
	if err != nil {
		return bmp
	}
	return out.ToNRGBA()
}

// bitmapIsSource reports whether the engine's bitmap still has exactly the
// source's pixels — no CSS or viewport resize happened.
func bitmapIsSource(li *engine.LoadedImage) bool {
	b := li.Bitmap.Bounds()
	return li.SourceW > 0 && b.Dx() == li.SourceW && b.Dy() == li.SourceH
}

// jpegPassable reports whether data is a JPEG a PDF reader renders as-is
// from a plain DCTDecode stream: one (grey) or three (YCbCr) components.
func jpegPassable(data []byte) bool {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	switch cfg.ColorModel {
	case color.GrayModel, color.YCbCrModel:
		return true
	}
	return false
}

// isOpaque reports whether every pixel has full alpha — cheap on the NRGBA
// the engine produces, pixel-by-pixel otherwise.
func isOpaque(img image.Image) bool {
	if n, ok := img.(*image.NRGBA); ok {
		for y := 0; y < n.Rect.Dy(); y++ {
			row := n.Pix[y*n.Stride : y*n.Stride+n.Rect.Dx()*4]
			for i := 3; i < len(row); i += 4 {
				if row[i] != 0xFF {
					return false
				}
			}
		}
		return true
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xFFFF {
				return false
			}
		}
	}
	return true
}

// svgPrintDPI is the density an SVG is rendered at when the caller named
// none. Unlike a photograph, an SVG has no pixels to "keep", so Options.
// ImageDPI's zero value — keep every pixel the source has — says nothing
// here and this stands in for it. 150 is the value Options.ImageDPI's own
// documentation calls a sound print figure: on A0 it puts the 21 px labels
// inside a schematic at some 33 px tall, and it is ~2.4x the pixels of the
// engine's 96 dpi raster rather than the ~10x that 300 would cost on a
// drawing the width of the page.
const svgPrintDPI = 150

// svgAtDensity re-renders an SVG element into the rectangle it is painted
// into, at imageDPI when the caller set one and at svgPrintDPI otherwise.
//
// The engine rasterises an SVG once, at the element's CSS box, for a canvas
// where one CSS px is one device px — a hard 96 dpi ceiling on paper, and
// the drawings that hit it are exactly the ones that most need to be crisp:
// schematics, charts, diagrams. Unlike a photograph there is nothing lost to
// recover, so this does not resample the bitmap: it renders the drawing
// again from its source (engine.RasterizeSVG), which is available for an
// inline <svg> as its own serialisation and for an <img src="*.svg"> as the
// bytes fetched.
//
// ok is false when there is no source to render from, when the rectangle is
// degenerate, or when the density asked for would gain nothing over the
// bitmap the engine already made — so a screen-density export keeps doing
// exactly what it did.
func svgAtDensity(li *engine.LoadedImage, r pdfkit.Rect, imageDPI float64) (image.Image, bool) {
	if li.Format != "svg" || len(li.Data) == 0 || r.Width <= 0 || r.Height <= 0 {
		return nil, false
	}
	dpi := imageDPI
	if dpi <= 0 {
		dpi = svgPrintDPI
	}
	w := int(math.Round(r.Width / 72 * dpi))
	h := int(math.Round(r.Height / 72 * dpi))
	if b := li.Bitmap.Bounds(); w <= b.Dx() || h <= b.Dy() {
		return nil, false
	}
	return engine.RasterizeSVG(li.Data, w, h, "")
}
