# html2pdf corpus run — 2026-09-06

8/8 succeeded.

| URL | Status | Pages | PDF | Text chars | Links | Fetch | Render |
|---|---|---|---|---|---|---|---|
| [https://example.com/](https://example.com/) | ✅ | 1 | 6887 B | 127 | 1 | 46ms | 33ms |
| [https://en.wikipedia.org/wiki/Go_(programming_language)](https://en.wikipedia.org/wiki/Go_(programming_language)) | ✅ | 13 | 251175 B | 55984 | 710 | 116ms | 492ms |
| [https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)](https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)) | ✅ | 7 | 125708 B | 19479 | 876 | 36ms | 715ms |
| [https://go.dev/blog/subtests](https://go.dev/blog/subtests) | ✅ | 5 | 60542 B | 12031 | 31 | 195ms | 669ms |
| [https://pkg.go.dev/net/http](https://pkg.go.dev/net/http) | ✅ | 51 | 383631 B | 145137 | 1790 | 593ms | 1375ms |
| [https://www.rfc-editor.org/rfc/rfc9110.html](https://www.rfc-editor.org/rfc/rfc9110.html) | ✅ | 85 | 1055265 B | 444905 | 2398 | 261ms | 565ms |
| [https://news.ycombinator.com/](https://news.ycombinator.com/) | ✅ | 1 | 28244 B | 3741 | 228 | 449ms | 797ms |
| [https://react.dev/](https://react.dev/) | ✅ | 8 | 7829423 B | 7737 | 158 | 115ms | 864ms |

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

### Inline decoration now painted — 2026-09-06 (engine [#128](https://github.com/go-webengine/engine/pull/128))

The last gap the README documented is closed at the source: go-webengine now
gives a styled inline element its own per-line box fragments
(`LineBox.Inlines []InlineFragment{Node, Style, X, Y, W, H, First, Last}`, in
document px, outermost element first) and reserves its horizontal
padding+border in the line advance. html2pdf paints each fragment through the
same `paintDecoration` path a block box uses — background, then borders with
`box-decoration-break: slice` semantics: the left border only on the
element's `First` fragment, the right only on its `Last`, top and bottom on
every fragment, all clipped to the page slice like a block's.

**Corpus effect: none on page counts, as it should be** — decoration is
paint, not layout. 7 of 8 pages keep their page count and text length to
within live-page noise; `react.dev` goes 12→13 pages / +61 extracted chars,
which is the *engine version bump itself* (its #127 stopped a flex container
mixing bare text with an element from silently dropping the text), not this
change. Five page-1 previews changed pixels for the same reason (engine #124
inline margins re-flowed RFC 9110's author block, for one) — none of the
corpus front pages has a decorated inline element to show the feature.

**So the proof is a synthetic page**, `fixtures/inline-decoration.html`
(reproduce line in the file): a span with background+border+padding wrapped
over two lines shows the left border on its first fragment only and the right
on its last only; a `<code>` badge gets a full box; a background pill paints
under a nested italic's `border-bottom`. One thing that looks like a defect
and isn't: the wrapped span's yellow bottom padding peeks out on the following
line under "and a", wherever the next line's own pill doesn't paint over it.
CSS does not grow a line box for an inline element's vertical padding, so it
overflows into the next line's band; a browser shows the same strip. Unit
proof: `TestExportPaintsInlineFragments` (the pinned engine really emits
fragments — exactly one First and one Last for a wrapped span — and the PDF
writes) and `TestPaintDecorationClipsToThePageSlice` (every clipping branch:
above / straddling top / inside / middle fragment / straddling bottom / below,
transparent background, zero width, nil style).

Not painted on inline elements, as on blocks here: `border-radius`,
`background-image`/gradients, `box-shadow` — flat fill, straight edges.

### Compressed streams — 2026-09-06

Found by [`BENCH.md`](BENCH.md), not by this corpus: against headless
Chrome on the same inputs, html2pdf's text-only PDFs were 6–16× larger
(RFC 9110 29.0 MB vs 4.5) — every content stream written raw, because
`Export` never set `pdfkit.Options.Compress`. Enabled since; the table above
is the first run with it. RFC 9110 28,979,365 → **4,683,687 B**, Wikipedia
(Go) 3,729,585 → 630,404, `pkg.go.dev/net/http` 8,634,499 → 1,289,572,
`react.dev` 5,215,046 → 4,789,356 (bitmaps, already flate — little to gain).
Page counts and text lengths unchanged; the output stays byte-deterministic
(`TestExportCompressesContentStreams`). Render time rises by the flate cost
on the big text pages (~0.3 s on RFC 9110).

### One `TJ` per run — 2026-09-06 (pdfkit [#28](https://github.com/go-pdfkit/pdfkit/pull/28))

Second finding from [`BENCH.md`](BENCH.md): with streams compressed, pure
text was still 2.7× Chrome's size because pdfkit's `TextShaped` positioned
every glyph with its own `Tm` + `Tj`. It now writes one `TJ` array per
baseline segment (numbers only where kerning or a mark departs from the
font's advances), stops rewriting unchanged font/colour state, and prints
four-decimal coordinates. Corpus bytes, previous run → this run: RFC 9110
4,683,687 → **981,952**, Wikipedia (Go) 630,404 → 215,610,
`pkg.go.dev/net/http` 1,289,572 → 385,845, countries list 228,222 → 78,288,
`react.dev` 4,789,356 → 4,734,109 (bitmaps). Pages and extracted text
unchanged.

### Navigation — links, named destinations, bookmarks, Title — 2026-09-06

The **Links** column counts the link annotations written (`/Subtype /Link`):
a URI action for an `http(s)` target, a GoTo to a named destination for a
fragment that points at an element id in the document; one rectangle per
line the anchor spans, or the box of an anchor that lays out no text. Every
element id is a named destination; the headings are the bookmark tree; the
`<title>` is the PDF's Title.

Verified on RFC 9110 (120 pages, 5 553 links) with the readers the judges
harness uses, not by counting strings:

| Check | Tool | Result |
|---|---|---|
| Title | poppler `pdfinfo` | `RFC 9110: HTTP Semantics` |
| Link kinds | pdf.js `getAnnotations` | 239 URI, 5 314 GoTo, 0 other |
| GoTo targets resolve | pdf.js `getDestination` + `getPageIndex` | 5 314 / 5 314 resolve to a page; e.g. the "Abstract" link on page 1 → page 1 at y = 628 pt |
| Bookmarks | MuPDF `mutool show … outline`, pdf.js `getOutline` | 311 items, nested by heading level, spread over 106 distinct pages; "1. Introduction" → page 1, "3.4. Messages" → 6, "8.8. Validator Fields" → 36, "B.3. Changes from RFC 7231" → 112 — each checked against `pdftotext` for the page the heading text is on |
| Structure | `qpdf --check` | no error |

**Hacker News: 198 links for 228 navigable anchors.** The 30 missing are all
the same shape: `<a href="vote?id=…"><div class="votearrow"></div></a>` — a
link whose only content is an empty block. The box fallback exists for
exactly this, but the arrow's 10 × 10 px come from HN's *external*
stylesheet (`news.css`), which this static renderer does not load (see
Scope), so the block lays out at 0.2 × 0 px and there is no rectangle to
give. A synthetic fixture with the same shape and an inline `style` gets its
rectangle (unit test `TestCollectLinksGivesAnAtomlessAnchorItsBoxRect`).
The anchors that are dropped on purpose — `mailto:`, `javascript:`,
fragments nobody anchors — are the rest of the gap to the raw `<a href>`
count; a dead link is worse than plain text.

### External stylesheets and the print medium — 2026-09-06 (engine [#133](https://github.com/go-webengine/engine/pull/133))

The "no external stylesheets" line in Scope was **self-inflicted**: the engine
had fetched `<link rel="stylesheet">` sheets (with `@import` chains, media
selection and bounds) for its own renders all along, and `Export` called
`css.Cascade(root)` with no sheets and the default viewport width. Engine
#133 exports that loader (`Engine.LoadStylesheets`, the way `LoadImages` was
exported for images) and adds `css.Media` so a cascade can be evaluated for
**print**: `@media print` blocks and `<link media="print">` apply, screen-only
ones do not. `Export` now does both, for `Options.Media` = print by default.

This is the largest fidelity change since images. Same eight pages, same
morning, before → after (before = #14's run):

| Page | Pages | Links | Text chars | PDF |
|---|---|---|---|---|
| Wikipedia (Go) | 18 → 13 | 1143 → 710 (Chrome 838) | 63 079 → 55 984 (Chrome 67 289) | 597 → 482 KB |
| countries table | 9 → 7 | 1119 → 876 (1481) | 22 951 → 19 479 (21 977) | 419 → 368 KB |
| go.dev/blog | 6 → 5 | 90 → 31 (15) | 13 481 → 12 031 (11 742) | 200 → 71 KB |
| pkg.go.dev | 49 → 51 | 2013 → 1790 (16 374) | 151 040 → 145 137 (150 480) | 826 → 735 KB |
| RFC 9110 | 120 → 85 | 5553 → 3398 (3506) | 449 848 → 444 905 (445 667) | 2.2 → 1.8 MB |
| Hacker News | 1 → 1 | 198 → 228 (260) | 3 757 → 3 802 (3 857) | 65 → 72 KB |
| react.dev | 13 → 8 | 140 → 158 (59) | 8 045 → 7 737 (6 809) | 4.8 → 7.9 MB |

What the eye sees (page-1 renders in `out/`): Wikipedia went from a dump of
its navigation lists in the UA's serif to the article as printed — serif
headings, the infobox at the right with the logo, superscript references,
sidebar and menus gone. RFC 9110 lost its table-of-contents sidebar and the
pilcrows (its print CSS hides both) and reflowed to the full column, which
is where 120 → 85 pages comes from; its link count is now within 3 % of
Chrome's. Hacker News has its orange bar, grey metadata and numbering, and
**all 228 navigable anchors** are clickable — the 30 vote arrows that #14
could not size are 10 × 10 px now that `news.css` is applied.

Every reader still passes with no warning (JUDGES.md). Consensus on
Wikipedia's page 1 rose 5.9 → 8.7 %: the page is now dense small text with
an infobox, where Ghostscript (Δ11.9 %) and Quartz (Δ14.2 %) anti-alias
heavier than poppler; MuPDF and pdf.js stay at 1–3 %. Same cause as before,
more of it on the page.

Two things this does not do yet: `@page` (margins, size, headers from CSS)
and `page-break-*` are not read — pagination is still the atom rule; and a
`@font-face` is still not fetched, so a site's own web font is set in the
bundled family of its generic (react.dev's page-1 render is the same text in
Inter).
