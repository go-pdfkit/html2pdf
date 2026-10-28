// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"strings"
	"testing"
)

// TestListMarkersEscapesCountersReachThePDF holds the three defects found on
// 2026-10-28 rendering a report: list markers not painted (with start= and
// list-style-type), CSS string escapes in content painted literally, and
// counter() painting nothing. Each must be text in the PDF, read back by the
// extractor — selectable, searchable — not only a shape on the page.
func TestListMarkersEscapesCountersReachThePDF(t *testing.T) {
	r := exportPDF(t, `<html><head><style>
.esc { list-style: none } .esc li::before { content: "\2022\00a0" }
ol.c { counter-reset: item; list-style: none }
ol.c > li::before { content: counter(item) ". "; counter-increment: item }
ol.n { list-style: none } ol.n li::before { content: counters(list-item, ".") " " }
</style></head><body>
<ul><li>bullet</li></ul>
<ol start="3"><li>three</li><li>four</li></ol>
<ol style="list-style-type: upper-roman"><li>one</li><li>two</li></ol>
<ol style="list-style-position: inside" start="7"><li>inside</li></ol>
<ul class="esc"><li>escaped</li></ul>
<ol class="c"><li>first</li><li>second</li></ol>
<ol class="n"><li>outer<ol class="n"><li>inner</li></ol></li></ol>
</body></html>`)
	// Compared without spaces: the extractor does not infer a space from the
	// gap between two drawn words ("plain words" reads "plainwords" too).
	text := pageText(t, r, 1)
	flat := strings.Join(strings.Fields(text), "")
	for _, want := range []string{
		"\u2022bullet",      // the ul's disc, as a glyph
		"3.three", "4.four", // ol start=3
		"I.one", "II.two", // upper-roman
		"7.inside",            // an inside marker: inline on the item's line
		"\u2022escaped",       // content: "\2022\00a0", escapes decoded
		"1.first", "2.second", // content: counter(item) ". "
		"1outer", "1.1inner", // counters() over nested lists
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the PDF's text lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `\2022`) || strings.Contains(text, "counter(") {
		t.Errorf("an escape or a counter() was painted literally:\n%s", text)
	}
}

// A marker on a list item that a page break pushes to the next page is
// painted on that page, not the first.
func TestListMarkerFollowsItsItemAcrossPages(t *testing.T) {
	r := exportPDF(t, `<html><body><ol><li>first</li><li style="break-before:page">second</li></ol></body></html>`)
	if r.PageCount() != 2 {
		t.Fatalf("PageCount = %d, want 2", r.PageCount())
	}
	if p1, p2 := pageText(t, r, 1), pageText(t, r, 2); !strings.Contains(p1, "1.") || strings.Contains(p1, "2.") ||
		!strings.Contains(p2, "2.") || strings.Contains(p2, "1.") {
		t.Errorf("page 1 %q, page 2 %q", p1, p2)
	}
}
