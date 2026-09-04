// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"

	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
)

// dataPNG returns a solid-colour w×h PNG as a data: URI, so image tests run
// entirely offline through the engine's real fetch/decode pipeline.
func dataPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 200, 30, 30, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestExportEmbedsAnImage(t *testing.T) {
	html := `<html><body style="margin:0"><p>before</p>` +
		`<img src="` + dataPNG(t, 40, 30) + `">` +
		`<p>after</p></body></html>`
	doc, err := Export(html, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// pdfkit writes every embedded bitmap as an image XObject.
	if !bytes.Contains(buf.Bytes(), []byte("/Subtype /Image")) {
		t.Error("no image XObject in output: the <img> was not embedded")
	}
}

func TestExportInlineSVGIsRasterisedAndEmbedded(t *testing.T) {
	// Inline <svg> rides the same engine pipeline (serialised then
	// rasterised), so it lands as a bitmap too — closing the gap the README
	// used to document.
	html := `<html><body style="margin:0">` +
		`<svg width="50" height="20" xmlns="http://www.w3.org/2000/svg">` +
		`<rect x="0" y="0" width="50" height="20" fill="#2954C8"/></svg>` +
		`</body></html>`
	doc, err := Export(html, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/Subtype /Image")) {
		t.Error("no image XObject in output: the inline <svg> was not embedded")
	}
}

func TestExportSkipsAnUnfetchableImage(t *testing.T) {
	// A src that can't be fetched is left out of the maps by the engine —
	// the document still exports, with no image XObject and no panic. Uses an
	// unroutable scheme so this never touches the network.
	html := `<html><body style="margin:0"><img src="nope://x/y.png"><p>text</p></body></html>`
	doc, err := Export(html, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("/Subtype /Image")) {
		t.Error("an image XObject appeared for an unfetchable src")
	}
}

func TestExportRelativeImageResolvesAgainstBaseURL(t *testing.T) {
	// With no BaseURL a relative src has nothing to resolve against and is
	// skipped rather than crashing; the export still succeeds. (A real
	// resolution round-trip needs the network and lives in corpus/.)
	html := `<html><body style="margin:0"><img src="pic.png"><p>t</p></body></html>`
	if _, err := Export(html, Options{}); err != nil {
		t.Fatalf("Export without BaseURL: %v", err)
	}
	if _, err := Export(html, Options{BaseURL: "https://x.test/dir/"}); err != nil {
		t.Fatalf("Export with BaseURL: %v", err)
	}
}

func TestPaintImageIgnoresAnItemWithNoBitmap(t *testing.T) {
	// Defensive: an image item whose element has no decoded bitmap must draw
	// nothing and not dereference a nil map entry (a nil imgs map is the
	// case where every fetch failed).
	e := &exporter{imgs: nil, scale: 1, pageHPt: 800, marginPt: 10}
	e.paintImage(&layout.InlineItem{Image: &dom.Node{}, ImgW: 10, ImgH: 10})
}
