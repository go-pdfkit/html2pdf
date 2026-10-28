// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
)

func exportPDF(t *testing.T, src string) *reader.Document {
	t.Helper()
	doc, err := Export(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	r, err := reader.Open(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func pageText(t *testing.T, r *reader.Document, n int) string {
	t.Helper()
	s, err := extract.Text(r, n)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(s), " ")
}

const threePages = `<div>one</div><div style="break-before:page">two</div><div style="break-before:page">three</div>`

func TestMarginBoxPageNumbers(t *testing.T) {
	r := exportPDF(t, `<html><head><style>
@page { size: A4; margin: 20mm;
  @bottom-center { content: counter(page) " / " counter(pages); font-size: 9pt }
}</style></head><body>`+threePages+`</body></html>`)
	if r.PageCount() != 3 {
		t.Fatalf("PageCount = %d, want 3", r.PageCount())
	}
	for i, want := range []string{"1 / 3", "2 / 3", "3 / 3"} {
		if got := pageText(t, r, i+1); !strings.Contains(got, want) {
			t.Errorf("page %d text %q lacks %q", i+1, got, want)
		}
	}
}

func TestMarginBoxesTopAndStrings(t *testing.T) {
	r := exportPDF(t, `<html><head><style>
@page { margin: 25mm;
  @top-left { content: "Left\2022" }
  @top-right { content: "Page " counter(page); text-align: right }
  @bottom-left { content: "BL" } @bottom-right { content: "BR" }
}</style></head><body>`+threePages+`</body></html>`)
	got := pageText(t, r, 2)
	for _, want := range []string{"Left•", "Page 2", "BL", "BR"} {
		if !strings.Contains(got, want) {
			t.Errorf("page 2 text %q lacks %q", got, want)
		}
	}
}

func TestNoMarginBoxWithoutContent(t *testing.T) {
	r := exportPDF(t, `<html><head><style>@page { @bottom-center { content: none } @top-center { color: red } }</style></head><body>x</body></html>`)
	if got := pageText(t, r, 1); got != "x" {
		t.Errorf("page text = %q, want just the body", got)
	}
}

func TestTargetCounterTOC(t *testing.T) {
	r := exportPDF(t, `<html><head><style>
a.toc::after { content: " p." target-counter(attr(href url), page); }
</style></head><body>
<p><a class="toc" href="#a">Alpha</a></p>
<p><a class="toc" href="#c">Gamma</a></p>
<p><a class="toc" href="#nothing">Missing</a></p>
<div id="a" style="break-before:page">alpha</div>
<div style="break-before:page">beta</div>
<div id="c" style="break-before:page">gamma</div>
</body></html>`)
	if r.PageCount() != 4 {
		t.Fatalf("PageCount = %d, want 4", r.PageCount())
	}
	got := strings.ReplaceAll(pageText(t, r, 1), " ", "")
	for _, want := range []string{"Alphap.2", "Gammap.4", "Missing"} {
		if !strings.Contains(got, want) {
			t.Errorf("page 1 text %q lacks %q", got, want)
		}
	}
	if strings.Contains(got, "Missingp.") {
		t.Errorf("a reference to a missing id printed a number: %q", got)
	}
}

func TestTargetCounterWithLeaderAndMargin(t *testing.T) {
	r := exportPDF(t, `<html><head><style>
@page { @bottom-center { content: counter(page) } }
li a::after { content: leader('.') target-counter(attr(href url), page) }
</style></head><body><ul><li><a href="#x">X</a></li></ul>
<div style="break-before:page"></div><h2 id="x">X heading</h2></body></html>`)
	if got := pageText(t, r, 1); !strings.Contains(got, "X 2") && !strings.Contains(got, "X2") {
		t.Errorf("page 1 text %q lacks the target page", got)
	}
}

func TestParseContent(t *testing.T) {
	got := parseContent(`"a\2022 b" counter(page) " / " counter(pages) leader('.') target-counter(attr(href url), page) unknown(1) 'q'`)
	want := []contentPart{
		{kind: partLit, lit: "a•b"}, {kind: partPage}, {kind: partLit, lit: " / "}, {kind: partPages},
		{kind: partTarget, attr: "href"}, {kind: partLit, lit: "q"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseContent = %#v\nwant %#v", got, want)
	}
	for _, none := range []string{"", "none", "normal", "counter(chapter)"} {
		if p := parseContent(none); len(p) != 0 {
			t.Errorf("parseContent(%q) = %v, want nothing", none, p)
		}
	}
}

func TestUnescapeCSS(t *testing.T) {
	for in, want := range map[string]string{
		`\2022`: "•", `\2022 x`: "•x", `a\"b`: `a"b`, "a\\\nb": "ab", `\41`: "A", `\110000`: "\uFFFD", `tail\`: `tail\`,
	} {
		if got := unescapeCSS(in); got != want {
			t.Errorf("unescapeCSS(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPageBoxDeclsHonoursMediaAndOrder(t *testing.T) {
	src := `/* c */ @page { @bottom-center { content: "a"; color: red } }
@media screen { @page { @bottom-center { content: "screen" } } }
@media print { @page { @bottom-center { content: "b" } } }
@page :first { @bottom-center { content: "first" } }`
	got := pageBoxDecls([]string{src}, css.Media{Type: css.Print})
	if c := got["bottom-center"]["content"]; c != `"b"` {
		t.Errorf("content = %q, want the later print rule", c)
	}
	if c := got["bottom-center"]["color"]; c != "red" {
		t.Errorf("color = %q, want it kept from the earlier rule", c)
	}
}

func TestTargetCounterSheet(t *testing.T) {
	root, err := dom.Parse(`<a href="#a">A</a><a href="#b">B</a><a href="#q&quot;x">Q</a>`)
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]int{"a": 3, "b": 12}
	got := targetCounterSheet([]string{`a.toc::after, .x:before { content: "p" target-counter(attr(href url), page) }`},
		css.Media{Type: css.Print}, root, func(id string) (int, bool) { n, ok := pages[id]; return n, ok })
	for _, want := range []string{
		`a.toc[href="#a"]::after{content:"p3"}`, `a.toc[href="#b"]::after{content:"p12"}`,
		`.x[href="#a"]:before{content:"p3"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sheet lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, `q"x`) {
		t.Errorf("an attribute value with a quote reached the sheet:\n%s", got)
	}
}

func TestParseColor(t *testing.T) {
	for in, want := range map[string][3]float64{
		"#f00": {1, 0, 0}, "#00ff00": {0, 1, 0}, "rgb(0, 0, 255)": {0, 0, 1}, "gray": {.5, .5, .5},
	} {
		c, ok := parseColor(in)
		if !ok || c.R != want[0] || c.G != want[1] || c.B != want[2] {
			t.Errorf("parseColor(%q) = %v, %v", in, c, ok)
		}
	}
	for _, bad := range []string{"", "#12", "rgb(1,2)", "notacolor"} {
		if _, ok := parseColor(bad); ok {
			t.Errorf("parseColor(%q) accepted", bad)
		}
	}
}

// FuzzPagedCSS: the paged-media readers take author CSS, so none of them may
// panic or loop on any input.
func FuzzPagedCSS(f *testing.F) {
	for _, s := range []string{
		`@page { @bottom-center { content: counter(page) " / " counter(pages) } }`,
		`a::after { content: target-counter(attr(href url), page) }`,
		`"\2022 \`, `@media print { @page {`, `/* open`, `content: "unterminated`, `a:before,,{content:target-counter(attr(`,
	} {
		f.Add(s)
	}
	root, _ := dom.Parse(`<a href="#a">x</a>`)
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<14 {
			t.Skip()
		}
		m := css.Media{Type: css.Print}
		_ = buildMarginBoxes(pageBoxDecls([]string{s}, m))
		_ = parseContent(s)
		_ = targetCounterSheet([]string{s}, m, root, func(string) (int, bool) { return 1, true })
		_, _ = parseColor(s)
		_, _ = lengthPt(s)
	})
}
