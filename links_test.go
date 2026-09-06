// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
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
	// No optional "\r" before "endstream": flate data can end in 0x0D, and
	// eating it truncates the zlib trailer (see corpus/internal/pdfstat).
	for _, m := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\nendstream`).FindAllSubmatch(pdf, -1) {
		if r, err := zlib.NewReader(bytes.NewReader(m[1])); err == nil {
			body, _ := io.ReadAll(r)
			out = append(append(out, '\n'), body...)
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
