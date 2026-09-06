# html2pdf

[![CI](https://github.com/go-pdfkit/html2pdf/actions/workflows/ci.yml/badge.svg)](https://github.com/go-pdfkit/html2pdf/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-pdfkit/html2pdf.svg)](https://pkg.go.dev/github.com/go-pdfkit/html2pdf)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

Pure-Go, **zero-C**, static HTML to **vector** PDF. No headless browser, no
screenshot-then-slice: it drives [go-webengine](https://github.com/go-webengine/engine)'s
own layout tree straight into [go-pdfkit](https://github.com/go-pdfkit/pdfkit)
text/rect/stroke calls, so the PDF's text is real text — selectable,
searchable, small — not a raster of it.

## Quick start

```go
doc, err := html2pdf.Export(htmlSource, html2pdf.Options{})
if err != nil {
    log.Fatal(err)
}
f, _ := os.Create("out.pdf")
defer f.Close()
doc.Write(f)
```

Or from the shell:

```sh
go run ./cmd/html2pdf -in report.html -out report.pdf
```

## Pagination

Page breaks land between atoms — a text line, or a whole `<tr>` — never
through one. A table row that would overflow the page moves to the next page
whole; a paragraph may still break between its own lines, same as printed
text always has.

## Layout width vs. print width

A page is laid out at `Options.ViewportPx` (default 1024px), then scaled down
to fit the print column — not laid out directly at the print column's own
width (a plain A4 page is under 650px wide). Many real pages carry a
fixed-width element sized for a desktop viewport (a sidebar, a multi-column
nav) that a browser's own responsive CSS only collapses below some
breakpoint; laying out narrower than that breakpoint just squeezes the rest
of the page into a sliver instead of dropping the sidebar. Confirmed against
RFC 9110's HTML edition, whose table-of-contents sidebar did exactly this —
see [`corpus/CORPUS.md`](corpus/CORPUS.md) for the before/after page counts
across all 8 corpus pages.

## Scope

This renders **static** HTML: no JavaScript, no external stylesheets, no
`@font-face`. Text is set in the three families go-webengine's own paint
package bundles — Inter (sans), Lora (serif), Go Mono (mono) — so the glyphs
drawn always match the metrics the layout pass measured against; there is no
web-font fetch to fail silently.

Images — raster `<img>`, `<img src="*.svg">` and inline `<svg>` — go through
the engine's own fetch/decode/size pipeline (`Engine.LoadImages`) and are
embedded as bitmaps, so they're laid out and drawn exactly as the engine's
raster canvas would draw them. A relative `src` resolves against
`Options.BaseURL`; an image that fails to fetch or decode is simply left out,
as on the raster canvas. This is the one place `Export` touches the network.

Inline-level decoration — a styled `<span>`, `<code>`, `<a>`… with a
background, border or padding — paints too, fragmented per line the way CSS
does (`box-decoration-break: slice`: the left border only on an element's
first fragment, the right only on its last). The geometry comes from the
engine's own `LineBox.Inlines` (go-webengine/engine#128), so a badge or pill
lands exactly where the raster canvas puts it. Not painted on inline
elements: `border-radius`, `background-image`/gradients, `box-shadow` — a
fragment paints a flat background and straight borders.

## Status

Validated against a hand-built regression suite (`html2pdf_test.go`, ~94%
statement coverage) and a corpus of 8 real public pages
([`corpus/`](corpus/), in the spirit of go-webengine's own
[`bench/`](https://github.com/go-webengine/engine/tree/main/bench)) — see
[`corpus/CORPUS.md`](corpus/CORPUS.md) for current results and the bugs the
corpus run has found so far.

## License

BSD-3-Clause, see [LICENSE](LICENSE).
