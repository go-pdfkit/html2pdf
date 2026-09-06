// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Hacker News shape that showed the gap: an anchor whose only content is
// an empty block sized by an EXTERNAL stylesheet. With the sheet applied the
// block is 10×10 px and the anchor gets a link rectangle; without it the
// block is 0×0 and there is nothing to click. The sheet also hides the
// anchor's container under @media print, so the default (print) medium
// writes no link and "screen" writes one — which proves both the fetch and
// the medium in one fixture.
func TestExportAppliesExternalStylesheetForTheRequestedMedium(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/s.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(`.votearrow{width:10px;height:10px;background:#f60}
@media print{.nav{display:none}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	withLink := `<html><head><link rel="stylesheet" href="/s.css"></head><body>
<div class="nav"><a href="https://example.com/vote"><div class="votearrow"></div></a></div>
<p>body text</p></body></html>`
	withoutLink := `<html><body>
<div class="nav"><a href="https://example.com/vote"><div class="votearrow"></div></a></div>
<p>body text</p></body></html>`

	links := func(html, media string) int {
		return bytes.Count(exportBytes(t, html, Options{BaseURL: srv.URL + "/", Media: media}), []byte("/Subtype /Link"))
	}
	if n := links(withLink, "screen"); n != 1 {
		t.Errorf("screen, external sheet: %d links, want 1 (the sheet sizes the vote arrow)", n)
	}
	if n := links(withLink, ""); n != 0 {
		t.Errorf("print (default), external sheet: %d links, want 0 (@media print hides the nav)", n)
	}
	if n := links(withoutLink, "screen"); n != 0 {
		t.Errorf("screen, no sheet: %d links, want 0 (an unsized block has no rectangle)", n)
	}
}

// A print-only <link media="print"> is applied under the default medium and
// skipped under screen — the reverse selection of the test above, on the
// link's own media attribute rather than an @media block.
func TestExportSelectsLinkMediaAttribute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/print.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(`.votearrow{width:10px;height:10px}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	// The anchor sits in a block container: an empty block directly under
	// <body> is laid out 8 px tall (the body margin) by the engine, which
	// would give the fallback a rectangle even with no sheet.
	html := `<html><head><link rel="stylesheet" href="/print.css" media="print"></head><body>
<div><a href="https://example.com/vote"><div class="votearrow"></div></a></div><p>body text</p></body></html>`
	if n := bytes.Count(exportBytes(t, html, Options{BaseURL: srv.URL + "/"}), []byte("/Subtype /Link")); n != 1 {
		t.Errorf("print: %d links, want 1 (the print-only sheet applies)", n)
	}
	if n := bytes.Count(exportBytes(t, html, Options{BaseURL: srv.URL + "/", Media: "screen"}), []byte("/Subtype /Link")); n != 0 {
		t.Errorf("screen: %d links, want 0 (the print-only sheet is skipped)", n)
	}
}
