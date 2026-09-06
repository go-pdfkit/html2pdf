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
//   - a JPEG source whose bitmap the engine did not resize is embedded as it
//     is — a DCTDecode stream of the original bytes, the rule Skia (Chrome's
//     PDF backend) applies; only YCbCr and grey JPEGs qualify, a CMYK JPEG
//     would need an inverted Decode array Adobe writers expect;
//   - any other lossy source (a resized JPEG, a lossy WebP) that is opaque
//     is re-encoded as JPEG at jpegQuality — lossy again, as Chrome does,
//     rather than stored losslessly at ten times the size;
//   - everything else — PNG, GIF, SVG rasters, anything with transparency —
//     is a flate bitmap with a soft mask when it has alpha, so line art and
//     screenshots keep every pixel.
func (e *exporter) paintImage(it *layout.InlineItem) {
	li, ok := e.imgs[it.Image]
	if !ok || li == nil || li.Bitmap == nil || it.ImgW <= 0 || it.ImgH <= 0 {
		return
	}
	x0, y0 := e.toPdf(it.X, it.Y)
	x1, y1 := e.toPdf(it.X+it.ImgW, it.Y+it.ImgH)
	r := pdfkit.Rect{X: x0, Y: y1, Width: x1 - x0, Height: y0 - y1}

	bmp := li.Bitmap
	if e.imageDPI > 0 {
		bmp = downsampleFor(bmp, r, e.imageDPI)
	}
	if li.Format == "jpeg" && bmp == li.Bitmap && bitmapIsSource(li) && jpegPassable(li.Data) {
		if e.p.DrawJPEG(li.Data, r) == nil {
			return
		}
	}
	if li.Lossy && isOpaque(bmp) {
		var buf bytes.Buffer
		if jpeg.Encode(&buf, bmp, &jpeg.Options{Quality: jpegQuality}) == nil && e.p.DrawJPEG(buf.Bytes(), r) == nil {
			return
		}
	}
	e.p.DrawImage(bmp, r)
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
