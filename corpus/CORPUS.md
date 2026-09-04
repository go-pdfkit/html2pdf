# html2pdf corpus run — 2026-09-04

8/8 succeeded.

| URL | Status | Pages | PDF | Text chars | Fetch | Render |
|---|---|---|---|---|---|---|
| [https://example.com/](https://example.com/) | ✅ | 1 | 26601 B | 127 | 38ms | 31ms |
| [https://en.wikipedia.org/wiki/Go_(programming_language)](https://en.wikipedia.org/wiki/Go_(programming_language)) | ✅ | 18 | 3737845 B | 63064 | 140ms | 244ms |
| [https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)](https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)) | ✅ | 9 | 1402676 B | 22953 | 43ms | 595ms |
| [https://go.dev/blog/subtests](https://go.dev/blog/subtests) | ✅ | 6 | 984773 B | 13478 | 166ms | 601ms |
| [https://pkg.go.dev/net/http](https://pkg.go.dev/net/http) | ✅ | 49 | 8656699 B | 150974 | 207ms | 1109ms |
| [https://www.rfc-editor.org/rfc/rfc9110.html](https://www.rfc-editor.org/rfc/rfc9110.html) | ✅ | 120 | 28978109 B | 449957 | 241ms | 464ms |
| [https://news.ycombinator.com/](https://news.ycombinator.com/) | ✅ | 1 | 257685 B | 4074 | 467ms | 471ms |
| [https://react.dev/](https://react.dev/) | ✅ | 12 | 9095777 B | 7977 | 47ms | 558ms |

<!-- BEGIN ANALYSIS -->

## Analysis

**8/8 rendered without a panic or a hung fetch** — the headline robustness
signal. Numbers above are from the run *after* the one bug this corpus found.

### Bug found and fixed: nested `<table>` swallowed whole (Hacker News)

First run: `news.ycombinator.com` produced 3 pages, page 1 blank below the
header, and only 1363 characters of extracted text. HN's markup is the
classic layout-table trick — the entire story list is one `<tr><td>` whose
cell holds a *second*, real `<table>` of 30 story rows. `collectAtoms`
treated any `<tr>` as one indivisible pagination atom without recursing into
it, so that outer row became a single atom the height of the *whole nested
table* — taller than a page, so it could only start at a page top, and
everything before it was wasted blank space on page 1.

Fix: a `<tr>` only counts as one atom when it has no `<tr>` *inside* it
(`hasDescendantTr` in `html2pdf.go`); a layout-table wrapper row is descended
into instead, so its real rows become the atoms. After the fix: 4 pages,
content from the top of page 1, 4009 characters extracted (~3×). Regression
test: `TestExportNestedLayoutTableSplitsAcrossPages`.

### Fixed: narrow print column vs. desktop-only responsive CSS (`Options.ViewportPx`)

`rfc-editor.org`'s RFC 9110 page rendered technically correctly but
inefficiently: 428 pages for a document whose official PDF runs closer to
180. The page's table-of-contents sidebar sits *beside* the article in a
fixed-width column with no breakpoint that drops it below desktop width, so
laying out directly at html2pdf's print column (170mm, ≈642px) squeezed the
prose to under half the page width — roughly double the line count it needed.

Fix: `Export` now lays out at a wider virtual viewport (`Options.ViewportPx`,
default 1024px) and scales the whole page down to fit the print column,
same idea as a browser print dialog's "shrink to fit". Result, this run vs.
the one that found the problem:

| Page | Before | After |
|---|---|---|
| RFC 9110 | 428 pages | **120 pages** |
| `pkg.go.dev/net/http` | 86 | 49 |
| Wikipedia (Go) | 34 | 18 |
| Wikipedia (countries list) | 17 | 9 |
| `go.dev/blog` | 9 | 6 |
| Hacker News | 4 | 2 |
| `react.dev` | 7 | 5 |

Extracted text length stayed within 1% on every page — this is a layout
density change, not a content change. `out/www-rfc-editor-org-rfc-rfc9110-html-p1-001.png`
and `out/news-ycombinator-com-p1-1.png` after the fix both show full-width,
readable text at a normal size — the scale-down doesn't make anything too
small to read at these ratios (642/1024 ≈ 0.63×).

### Images now painted — `<img>`, `<img src="*.svg">` and inline `<svg>`

Both image gaps the README used to document are closed in one step. Rather
than re-implement fetch/decode/budgeting, `Export` calls the engine's own
pipeline — `Engine.LoadImages`, exported for exactly this in
go-webengine/engine#114 — and hands its intrinsic-size map to
`layout.LayoutDocument` and its bitmap map to the PDF painter. So an image is
laid out at, and drawn at, precisely the size the engine's raster canvas would
use; an inline `<svg>` arrives already rasterised by the same path.

Embedded image XObjects per page, this run: Wikipedia (Go) 30, countries
list 10, `go.dev/blog` 40, Hacker News 6, `pkg.go.dev/net/http` 88,
`react.dev` 166, `example.com` 0 and RFC 9110 0 (neither carries an `<img>`).
Wikipedia's logo and react.dev's logo/icons land in the right place at the
right size on their page-1 previews. Text length is unchanged everywhere.

Two things to read correctly in the table above after this change:

- **`react.dev` grew 5→12 pages and 0.5→9 MB.** That is 166 icon-sized
  inline SVGs, each rasterised and stored as raw FlateDecode RGB samples —
  pdfkit does not JPEG-encode or deduplicate identical bitmaps. Correct, but
  heavy; deduplicating repeated bitmaps into one shared XObject is the
  obvious next saving.
- **Render times rose** (`pkg.go.dev` 177→1109 ms, countries 143→595 ms):
  image fetch + decode now runs inside `Export`, so the Render column is
  network-bound for image-bearing pages. The Fetch column still measures only
  the page's own HTML.

### Confirmed-expected: `react.dev` shows only its static shell

7 pages of real content (nav, hero, first section, one code sample) — the
part of the page that ships in the HTML itself. No JavaScript runs, so
nothing that hydrates client-side appears. This is the documented scope
limitation working as expected, not a failure.
