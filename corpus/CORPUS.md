# html2pdf corpus run — 2026-09-06

8/8 succeeded.

| URL | Status | Pages | PDF | Text chars | Fetch | Render |
|---|---|---|---|---|---|---|
| [https://example.com/](https://example.com/) | ✅ | 1 | 26601 B | 127 | 40ms | 33ms |
| [https://en.wikipedia.org/wiki/Go_(programming_language)](https://en.wikipedia.org/wiki/Go_(programming_language)) | ✅ | 18 | 3732448 B | 63064 | 114ms | 217ms |
| [https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)](https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)) | ✅ | 9 | 1402676 B | 22953 | 37ms | 495ms |
| [https://go.dev/blog/subtests](https://go.dev/blog/subtests) | ✅ | 6 | 978154 B | 13478 | 173ms | 516ms |
| [https://pkg.go.dev/net/http](https://pkg.go.dev/net/http) | ✅ | 49 | 8634363 B | 150974 | 199ms | 1116ms |
| [https://www.rfc-editor.org/rfc/rfc9110.html](https://www.rfc-editor.org/rfc/rfc9110.html) | ✅ | 120 | 28978109 B | 449957 | 201ms | 452ms |
| [https://news.ycombinator.com/](https://news.ycombinator.com/) | ✅ | 1 | 240064 B | 3792 | 455ms | 489ms |
| [https://react.dev/](https://react.dev/) | ✅ | 12 | 5215737 B | 7977 | 126ms | 603ms |

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
  pdfkit re-embedded a bitmap once per placement. Correct, but heavy;
  deduplicating repeated bitmaps into one shared XObject was the obvious next
  saving, and it has since landed in pdfkit — see **Bitmap dedup** below,
  which took this page to 5.2 MB.
- **Render times rose** (`pkg.go.dev` 177→1109 ms, countries 143→595 ms):
  image fetch + decode now runs inside `Export`, so the Render column is
  network-bound for image-bearing pages. The Fetch column still measures only
  the page's own HTML.

### Bitmap dedup — 2026-09-06 (pdfkit [#26](https://github.com/go-pdfkit/pdfkit/pull/26))

The saving named just above has landed upstream. pdfkit now content-addresses
every image by its **uncompressed** samples plus width, height, colour space,
bits per component and the alpha channel — the raw DCTDecode bytes for
`DrawJPEG` — and keeps that index on the `Document`, so a pixel-identical
bitmap is embedded, and flate-compressed, **once** and shared by every
placement on every page. html2pdf gets it for free by moving the dependency;
nothing in this repo changed but `go.mod`.

Both columns below were measured **the same morning, minutes apart**, against
the same live pages: one corpus run on `main`'s pdfkit (v0.11.0, no dedup) and
one on the merged commit. That matters here — these are public pages that can
change under the measurement, and the recorded numbers from 2026-09-04 would
have mixed two days' fetches into one comparison. Counts are
`grep -ac "/Subtype /Image" out/<slug>.pdf`, which counts each image's
`/SMask` companion stream as well as the image itself.

| Page | Bytes before | Bytes after | Δ | XObj before | XObj after |
|---|---|---|---|---|---|
| `react.dev` | 9,095,777 | **5,215,737** | **−42.7%** | 166 | **89** |
| `pkg.go.dev/net/http` | 8,656,699 | 8,634,363 | −0.3% | 88 | **44** |
| `go.dev/blog/subtests` | 984,773 | 978,154 | −0.7% | 40 | **24** |
| Wikipedia (Go) | 3,733,450 | 3,732,448 | −0.03%\* | 26 | 24\* |
| Wikipedia (countries list) | 1,402,676 | 1,402,676 | 0 | 10 | 10 |
| Hacker News | 240,064 | 240,064 | 0 | 4 | 4 |
| `example.com` | 26,601 | 26,601 | 0 | 0 | 0 |
| RFC 9110 | 28,978,109 | 28,978,109 | 0 | 0 | 0 |

\* **Wikipedia (Go) is the one row that is noise, not signal.** Two runs in the
*same* configuration gave 3,736,561 B / 26 streams and 3,732,448 B / 24 — the
page serves different image bytes from one fetch to the next, a ±4 KB spread
that swamps anything dedup does to it. Every other page reproduced to the byte
across repeated runs, which is what makes the rest of the column trustworthy.

Three things worth reading off this table:

- **`react.dev` is the whole story: 3.9 MB, 43% of the file, was one handful
  of icons written 166 times.** Those are icon-sized inline SVGs, rasterised
  and stored as raw FlateDecode RGB samples once per placement; 89 streams now
  carry the same picture.
- **Halving the stream count does not halve the bytes.** `pkg.go.dev` drops 88
  streams to 44 for only 22 KB, and `go.dev/blog` 40 to 24 for 6.6 KB: the
  duplicates there are small icons whose flate streams were already tiny. A
  count is not a size, and reporting only the count would have oversold this
  by two orders of magnitude on those two pages.
- **Nothing else moved.** Page counts and extracted-text lengths are identical
  on all eight pages, and *every* page-1 preview PNG is byte-identical between
  the before and after runs. Dedup is a change to the file's structure, not to
  what it draws — which is what pdfkit's own determinism test asserts, checked
  here against real pages.

### Confirmed-expected: `react.dev` shows only its static shell

7 pages of real content (nav, hero, first section, one code sample) — the
part of the page that ships in the HTML itself. No JavaScript runs, so
nothing that hydrates client-side appears. This is the documented scope
limitation working as expected, not a failure.
