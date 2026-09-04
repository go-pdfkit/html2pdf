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

```
go run ./cmd/html2pdf -in report.html -out report.pdf
```

## Pagination

Page breaks land between atoms — a text line, or a whole `<tr>` — never
through one. A table row that would overflow the page moves to the next page
whole; a paragraph may still break between its own lines, same as printed
text always has.

## Scope

This renders **static** HTML: no JavaScript, no external stylesheets, no
`@font-face`. Text is set in the three families go-webengine's own paint
package bundles — Inter (sans), Lora (serif), Go Mono (mono) — so the glyphs
drawn always match the metrics the layout pass measured against; there is no
web-font fetch to fail silently.

Two gaps, both inherited from — not introduced by — the layout engine:

- **Inline-level background/border/padding does not paint.** A styled
  `<span>` never gets its own box in go-webengine's layout (confirmed against
  its reference raster painter too — a shared engine limitation). Style the
  *containing* block/table-cell instead of an inner inline element when you
  need a filled badge or pill.
- **Inline `<svg>` and `<img>` are not painted yet.** Build charts from plain
  block/table markup (backgrounds, borders, percentage widths) rather than
  inline SVG until this lands.

## Status

Early — validated so far against a hand-built regression suite
(`html2pdf_test.go`) and one real multi-page report. A corpus run against
public real-world pages, in the spirit of go-webengine's own
[`bench/`](https://github.com/go-webengine/engine/tree/main/bench), is
tracked in [`corpus/`](corpus/) and [`CORPUS.md`](CORPUS.md).

## License

BSD-3-Clause, see [LICENSE](LICENSE).
