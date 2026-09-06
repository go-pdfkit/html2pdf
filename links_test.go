// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
	"github.com/go-webengine/engine/paint"
)

// exportBytes renders html and returns the PDF's bytes followed by the
// inflated body of every flate stream in it, so a test can look for a
// dictionary string whether it sits bare in the file or packed in a PDF 1.5
// object stream (Export writes those: annotations, destinations, the Info
// dictionary all live compressed).
func exportBytes(t *testing.T, html string, opts Options) []byte {
	t.Helper()
	doc, err := Export(html, opts)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	pdf := buf.Bytes()
	out := append([]byte(nil), pdf...)
	for _, m := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`).FindAllSubmatch(pdf, -1) {
		if r, err := zlib.NewReader(bytes.NewReader(m[1])); err == nil {
			if body, err := io.ReadAll(r); err == nil {
				out = append(append(out, '\n'), body...)
			}
			r.Close()
		}
	}
	return out
}

func TestExportWritesExternalLinkAsURIAction(t *testing.T) {
	out := exportBytes(t, `<html><body>see <a href="https://example.org/x?q=1">the page</a> now</body></html>`, Options{})
	for _, want := range []string{"/Subtype /Link", "/S /URI", "/URI (https://example.org/x?q=1)"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output lacks %q", want)
		}
	}
}

func TestExportInDocumentLinkBecomesGoToWithNamedDest(t *testing.T) {
	out := exportBytes(t, `<html><body>
		<p><a href="#sec">jump</a> and <a href="#nowhere">dangling</a></p>
		<h2 id="sec">Section</h2><p>body</p>
	</body></html>`, Options{})
	if n := bytes.Count(out, []byte("/S /GoTo")); n != 1 {
		t.Errorf("GoTo actions = %d, want 1 (the dangling #nowhere must not become a link)", n)
	}
	if !bytes.Contains(out, []byte("/Dests")) {
		t.Error("no /Dests name tree written for the id'd heading")
	}
}

func TestExportSameDocumentURLWithFragmentIsInternal(t *testing.T) {
	out := exportBytes(t, `<html><body><a href="https://host/doc#top">up</a><div id="top">x</div></body></html>`,
		Options{BaseURL: "https://host/doc"})
	if bytes.Contains(out, []byte("/S /URI")) {
		t.Error("a link to the document's own URL + fragment should be a GoTo, not a URI action")
	}
	if n := bytes.Count(out, []byte("/S /GoTo")); n != 1 {
		t.Errorf("GoTo actions = %d, want 1", n)
	}
}

func TestExportDropsNonNavigableLinks(t *testing.T) {
	out := exportBytes(t, `<html><body>
		<a href="javascript:void(0)">js</a> <a href="mailto:a@b">mail</a> <a href="tel:+1">tel</a> <a href="">empty</a> <a href="   ">blank</a>
	</body></html>`, Options{})
	if bytes.Contains(out, []byte("/Subtype /Link")) {
		t.Error("javascript:/mailto:/tel:/empty hrefs must not produce link annotations")
	}
}

func TestExportRelativeLinkResolvesAgainstBaseURL(t *testing.T) {
	out := exportBytes(t, `<html><body><a href="../other/page.html">rel</a></body></html>`,
		Options{BaseURL: "https://host/a/b/c.html"})
	if !bytes.Contains(out, []byte("/URI (https://host/a/other/page.html)")) {
		t.Errorf("relative href not resolved against BaseURL:\n%s", grepLines(out, "URI"))
	}
}

func TestCollectLinksSplitsAWrappedAnchorPerLine(t *testing.T) {
	// A link long enough to wrap in a narrow container yields one run per
	// line — the clickable area follows the text — not one bounding box.
	root, err := layoutAtWidthForTest(`<html><body style="margin:0"><div style="width:200px">`+
		`<a href="https://example.org/">`+strings.Repeat("word ", 40)+`</a></div></body></html>`, 1024)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://example.org/")
	runs := collectLinks(root, base, nil)
	if len(runs) < 2 {
		t.Fatalf("runs = %d, want one per wrapped line (>= 2)", len(runs))
	}
	for i := 1; i < len(runs); i++ {
		if runs[i].y <= runs[i-1].y {
			t.Errorf("run %d (y=%v) is not below run %d (y=%v)", i, runs[i].y, i-1, runs[i-1].y)
		}
		if runs[i].uri != runs[0].uri {
			t.Errorf("run %d target %q differs from %q", i, runs[i].uri, runs[0].uri)
		}
	}
}

func TestCollectIDsBlockAndInlineAndLegacyName(t *testing.T) {
	root, err := layoutAtWidthForTest(`<html><body style="margin:0">
		<p>intro</p>
		<div id="block">block</div>
		<p>text <span id="inline">inline</span> text <a name="legacy">old</a></p>
		<div id="block">duplicate</div>
	</body></html>`, 1024)
	if err != nil {
		t.Fatal(err)
	}
	ids := collectIDs(root)
	for _, id := range []string{"block", "inline", "legacy"} {
		if _, ok := ids[id]; !ok {
			t.Errorf("id %q not found", id)
		}
	}
	if ids["inline"].y <= ids["block"].y {
		t.Errorf("inline id (y=%v) should sit below the block id (y=%v)", ids["inline"].y, ids["block"].y)
	}
	if len(ids) != 3 {
		t.Errorf("ids = %v, want exactly 3 (duplicate id keeps the first)", ids)
	}
}

func TestExportOutlineFromHeadingsAndTitleFromDocument(t *testing.T) {
	out := exportBytes(t, `<html><head><title>  Mon   rapport </title></head><body>
		<h1>Chapter One</h1><p>a</p><h2>Part <b>Alpha</b></h2><p>b</p><h3></h3>
	</body></html>`, Options{Author: "Moi"})
	for _, want := range []string{"/Outlines", "(Chapter One)", "(Part Alpha)", "/Title (Mon rapport)", "/Author (Moi)"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output lacks %q", want)
		}
	}
	// An explicit Title wins over <title>.
	out = exportBytes(t, `<html><head><title>doc</title></head><body>x</body></html>`, Options{Title: "Given"})
	if !bytes.Contains(out, []byte("/Title (Given)")) || bytes.Contains(out, []byte("/Title (doc)")) {
		t.Error("Options.Title should override the document <title>")
	}
}

func TestPageIndexOf(t *testing.T) {
	tops := []float64{0, 100, 250}
	for _, tc := range []struct {
		y    float64
		want int
	}{{0, 0}, {99.9, 0}, {100, 1}, {249, 1}, {250, 2}, {1e6, 2}} {
		if got := pageIndexOf(tc.y, tops); got != tc.want {
			t.Errorf("pageIndexOf(%v) = %d, want %d", tc.y, got, tc.want)
		}
	}
}

func TestResolveAnchorEdgeCases(t *testing.T) {
	base, _ := url.Parse("https://host/dir/page")
	ids := map[string]struct{}{"here": {}}
	for _, tc := range []struct {
		raw        string
		wantOK     bool
		wantURI    string
		wantDest   string
		withNoBase bool
	}{
		{raw: "#here", wantOK: true, wantDest: "here"},
		{raw: "#gone", wantOK: false},
		{raw: "https://host/dir/page#here", wantOK: true, wantDest: "here"},
		{raw: "https://host/dir/page#gone", wantOK: false},
		{raw: "https://other/#here", wantOK: true, wantURI: "https://other/#here"},
		{raw: "sub/x", wantOK: true, wantURI: "https://host/dir/sub/x"},
		{raw: "://bad url", wantOK: false},
		{raw: "ftp://host/f", wantOK: false},
		{raw: "https://abs/x", wantOK: true, wantURI: "https://abs/x", withNoBase: true},
		{raw: "rel/x", wantOK: false, withNoBase: true}, // no base: a relative href resolves to nothing navigable
	} {
		b := base
		if tc.withNoBase {
			b = nil
		}
		got := resolveAnchor(b, tc.raw, ids)
		if got.ok != tc.wantOK || got.uri != tc.wantURI || got.dest != tc.wantDest {
			t.Errorf("resolveAnchor(%q) = %+v, want ok=%v uri=%q dest=%q", tc.raw, got, tc.wantOK, tc.wantURI, tc.wantDest)
		}
	}
}

// grepLines returns the lines of a PDF that mention s, for a failure message.
func grepLines(pdf []byte, s string) string {
	var out []string
	for _, l := range strings.Split(string(pdf), "\n") {
		if strings.Contains(l, s) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// An anchor whose only content is an empty styled block — a vote arrow, an
// icon drawn by CSS — lays out no atom, so the line pass finds nothing; the
// box pass gives it the block's own rectangle, once, and a text-bearing
// anchor is not doubled by it.
func TestCollectLinksGivesAnAtomlessAnchorItsBoxRect(t *testing.T) {
	html := `<html><body>
<a href="https://example.com/vote"><div style="width:10px;height:10px"></div></a>
<p><a href="https://example.com/text">one <span>two</span></a></p>
</body></html>`
	root, _ := dom.Parse(html)
	box, _ := layout.LayoutDocument(root, css.Cascade(root), 1024, paint.NewFonts(), nil)
	runs := collectLinks(box, nil, nil)
	var votes, texts int
	for _, r := range runs {
		switch r.uri {
		case "https://example.com/vote":
			votes++
			if r.w != 10 || r.h != 10 {
				t.Errorf("vote run = %vx%v at (%v,%v), want the 10x10 block", r.w, r.h, r.x, r.y)
			}
		case "https://example.com/text":
			texts++
		default:
			t.Errorf("unexpected run %+v", r)
		}
	}
	if votes != 1 || texts != 1 {
		t.Fatalf("runs: vote %d, text %d; want 1 and 1 (%d runs total)", votes, texts, len(runs))
	}
}
