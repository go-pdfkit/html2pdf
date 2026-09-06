# html2pdf vs headless Chrome — 2026-09-06

Machine: Apple M4 Max, 16 cores; load average at start: { 15.55 14.25 13.40 }. Chrome: Google Chrome 152.0.7977.76. 5 runs per tool per input, interleaved; medians. Both tools timed as child processes under `/usr/bin/time -l` (wall-clock to PDF on disk, start-up and any fetch included; RSS = peak resident set).

| Input | html2pdf | Chrome | Chrome ÷ html2pdf | PDF html2pdf | PDF Chrome | Pages | Text chars | Links | RSS html2pdf | RSS Chrome |
|---|---|---|---|---|---|---|---|---|---|---|
| https://example.com/ | 0.08 s | 2.37 s | 29.6× | 0.0 MB | 0.0 MB | 1 / 1 | 127 / 128 | 1 / 1 | 49.2 MB | 241.1 MB |
| https://en.wikipedia.org/wiki/Go_(programming_language) | 0.62 s | 2.98 s | 4.8× | 0.2 MB | 1.7 MB | 13 / 25 | 55984 / 67289 | 710 / 838 | 100.3 MB | 362.3 MB |
| https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations) | 0.75 s | 2.84 s | 3.8× | 0.1 MB | 1.7 MB | 7 / 12 | 19479 / 21977 | 876 / 1481 | 100.4 MB | 358.2 MB |
| https://go.dev/blog/subtests | 0.88 s | 3.87 s | 4.4× | 0.1 MB | 0.2 MB | 5 / 7 | 12031 / 11742 | 31 / 15 | 69.9 MB | 352.7 MB |
| https://pkg.go.dev/net/http | 1.61 s | 4.39 s | 2.7× | 0.4 MB | 6.3 MB | 51 / 86 | 145137 / 150480 | 1790 / 16374 | 119.9 MB | 677.3 MB |
| https://www.rfc-editor.org/rfc/rfc9110.html | 0.79 s | 4.50 s | 5.7× | 1.1 MB | 4.5 MB | 85 / 169 | 444905 / 445667 | 3398 / 3506 | 180.5 MB | 550.0 MB |
| https://news.ycombinator.com/ | 1.23 s | 3.27 s | 2.7× | 0.0 MB | 0.4 MB | 1 / 2 | 3657 / 3710 | 228 / 260 | 57.1 MB | 257.8 MB |
| https://react.dev/ | 0.99 s | 3.12 s | 3.2× | 7.8 MB | 2.7 MB | 8 / 9 | 7737 / 6809 | 158 / 59 | 251.6 MB | 301.4 MB |
| fixtures/longdoc.html | 0.41 s | 2.53 s | 6.2× | 0.9 MB | 2.4 MB | 100 / 135 | 590571 / 591170 | 0 / 0 | 144.7 MB | 372.3 MB |

<!-- BEGIN ANALYSIS -->

## Analysis — 2026-09-06

**Read the machine line first.** Load average was ~12 on 16 cores throughout
(a Syncthing scan, another session's measurement process and a ColorSync
agent, none of them mine to stop). Interleaving and medians make the two
columns comparable to *each other* under that load; the absolute seconds
would be lower on an idle machine. Spread across the 5 runs was tight for
both tools except html2pdf on `example.com` (0.07–0.54 s: one cold run) and
the countries list (0.54–1.16 s: one slow image fetch).

### Speed: 2–5× on real pages, and most of Chrome's time is not rendering

Chrome's floor is ~1.6 s: that is `example.com`, a page with nothing on it —
process start-up, profile, GPU-less compositor. Everything else it does costs
0.5–2 s on top. So "23× on example.com" is a start-up comparison, not a
rendering one; the honest headline is the **2.1–5.5× on real documents**,
where both tools actually work. A long-lived Chrome driven over DevTools
would amortise its start-up and narrow the gap; a one-shot CLI, which is what
`--print-to-pdf` is, cannot. The self-contained `longdoc` (no network for
either side) says the same thing: 0.80 s vs 2.13 s.

html2pdf's own cost is dominated by fetch on image-bearing pages
(`pkg.go.dev`: 1.69 s, its images fetched inside `Export`) and by flate on
the big text ones (RFC 9110 went 0.69 → 0.94 s when compression was turned
on — see below). Peak memory is 2–5× lower than Chrome's on every input;
html2pdf's grows with the document (the whole box tree is held: 240 MB for
RFC 9110 and longdoc), Chrome's floor is ~240 MB before it has laid out a
single line.

### Output size: one line fixed a 6–16× deficit; two honest residuals

The first run of this bench put html2pdf at **29.0 MB vs Chrome's 4.5 MB on
RFC 9110 and 37.7 vs 2.4 MB on `longdoc`** — text-only documents, so bitmap
dedup had nothing to do with it. Every content stream was being written raw:
`pdfkit.Options.Compress` exists and `Export` was not setting it (0
`FlateDecode` in a 37.7 MB file; 149 in Chrome's). Enabled since
(`TestExportCompressesContentStreams` guards it, and that the output stays
byte-deterministic). Result: RFC 9110 **4.7 MB**, Wikipedia 0.6 MB (Chrome
1.7), the countries list 0.2 MB (1.6), `pkg.go.dev` 1.3 MB (6.3) — html2pdf
now ships the *smaller* file on five of nine inputs.

Two inputs were still larger after compression, for different reasons — the
first is now fixed upstream (next section), the second remains:

- **`longdoc`, 6.5 vs 2.4 MB, no images**: how text was written. pdfkit's
  `TextShaped` positioned **every glyph** with its own absolute `Tm` + `Tj`
  (5,683 `Tj` for 748 text objects on one page — 411 KB decompressed against
  Chrome's 30 KB), at 17-digit coordinates, and every word re-set the same
  font and colour. Fixed in pdfkit
  [#28](https://github.com/go-pdfkit/pdfkit/pull/28): one `TJ` array per
  baseline segment, numbers only where shaping departs from the font's own
  advances, unchanged state not rewritten, four-decimal coordinates.
- **`react.dev`, 4.7 vs 2.7 MB**: bitmaps. Its icons are inline SVGs the
  engine rasterises, and html2pdf embeds them as flate-compressed RGB
  samples; Chrome keeps them as vector paths. Dedup already halved this page;
  keeping SVG vector would need a path exporter in the engine.

### Update — one `TJ` per run (pdfkit #28), same morning

The table above is this run. Against the previous one (compressed streams,
per-glyph `Tm`), same machine, load now ~12–17:

| Input | PDF before | PDF after | Chrome | html2pdf time before → after |
|---|---|---|---|---|
| RFC 9110 | 4.7 MB | **1.0 MB** | 4.5 MB | 0.94 → 0.78 s |
| `longdoc` | 6.5 MB | **0.9 MB** | 2.4 MB | 0.80 → 0.41 s |
| `pkg.go.dev/net/http` | 1.3 MB | 0.4 MB | 6.3 MB | 1.69 → 1.66 s |
| Wikipedia (Go) | 0.6 MB | 0.2 MB | 1.7 MB | 0.39 → 0.35 s |
| `react.dev` | 4.8 MB | 4.7 MB | 2.7 MB | 0.67 → 0.73 s |

html2pdf now writes the smaller file on eight of nine inputs — `longdoc`,
the pure-text case that was 2.7× *larger* than Chrome's, is 2.7× *smaller* —
and got faster on the text-heavy pages, there being less to flate. Page
counts and extracted text are unchanged (the same glyphs land in the same
places; only how the stream says so changed — pdfkit's own tests pin the pen
arithmetic for kerning, offsets and marks). `react.dev` is untouched, as
expected: it is bitmaps, not text.

### Pages and text: not a like-for-like layout, by design

html2pdf produces fewer pages on every text page (RFC 9110 120 vs 169,
`pkg.go.dev` 49 vs 86): `Options.ViewportPx` lays out at 1024 px and scales
to the print column (≈0.63×), so the type is smaller and the page denser;
Chrome prints at CSS px with no shrink. Set `ViewportPx` to the column width
for a 1:1 comparison. Extracted text agrees within ~5% except where Chrome
runs the page's JavaScript and print stylesheet and html2pdf does not
(`react.dev` 13 vs 9 pages, `go.dev/blog` hiding its nav in print CSS).

### Links — parity with Chrome is not a count — 2026-09-06

The **Links** column is `/Subtype /Link` annotations, html2pdf / Chrome. The
counts differ in both directions and neither side's number is a correctness
measure:

| Input | html2pdf | Chrome | Why they differ |
|---|---|---|---|
| Wikipedia (Go) | 1143 | 838 | Chrome prints under `@media print`, which hides the sidebar, TOC and edit links; we lay out the screen stylesheet (no `@media print` support yet) |
| go.dev/blog, react.dev | 90, 140 | 15, 59 | same: site navs hidden in print |
| RFC 9110 | 5553 | 3506 | same: the TOC sidebar's links hidden in print; ours are one rectangle per line, which also splits a wrapped anchor |
| countries table | 1119 | 1481 | Chrome writes one annotation per text fragment, so a flag + name cell is two; ours merges an anchor's atoms on a line into one |
| pkg.go.dev | 2013 | 16374 | Chrome ran the page's JavaScript, which builds the per-symbol index; this is a static renderer |
| Hacker News | 198 | 260 | 30 are vote arrows sized only by HN's external stylesheet, which we do not load (0 × 0 px, no rectangle); the rest are `mailto:` and per-fragment splits — see CORPUS.md |

What *is* a correctness measure was done in CORPUS.md: every GoTo resolves
in pdf.js, the bookmarks point at the pages the headings are on, every reader
still passes with no warning (JUDGES.md, rerun on these files).

**Size residual, stated plainly**: RFC 9110 went 1.0 MB → 2.2 MB for its
5 553 annotations, i.e. ~220 B each — every annotation is its own indirect
object with an uncompressed dictionary and an xref entry, which is how PDF
1.4 has to write it. Chrome's file (4.5 MB, 3 506 links) is still twice ours,
but the fix is known: PDF 1.5 *object streams* flate the non-stream objects
together, and belong in pdfkit, not here. `longdoc` (no links) is unchanged
at 0.9 MB.

### Two changes, measured one at a time — 2026-09-06

The afternoon's two changes were run through the harness separately, so each
one's effect is on record alone (commit "corpus/bench/judges rerun with
external stylesheets and print media" holds the first stage's tables).

**1. External stylesheets + print medium** (engine #133; see CORPUS.md for the
page-by-page table and the renders). For the bench this means the layout is
now the page's *printed* layout, so the pages / text / links columns are
closer to Chrome's than they have ever been — RFC 9110 3398 links vs 3506,
Hacker News 228 vs 260 — while what remains different is now nameable:
Chrome's extra pages come from printing at 1:1 rather than our 0.63×
ViewportPx scale; its extra links on pkg.go.dev from JavaScript-built
content; its fewer links on go.dev/blog and react.dev from per-fragment
splitting on their side vs per-line on ours. Time moved with the work: a
page whose stylesheet now lays out a real design costs more to cascade and
paint (Wikipedia 0.36 → 0.60 s, react.dev 0.73 → 1.07 s), a page that lost
its sidebar costs less to paginate (RFC 0.71 → 0.87 s includes fetching its
stylesheet). Peak RSS *fell* on the text pages (Wikipedia 109 → 97 MB,
pkg.go.dev 123 → 104 MB) and rose on react.dev (155 → 246 MB) — see below.

**2. Object streams** (pdfkit #29; `Options.ObjectStreams`, PDF 1.5). Every
annotation, destination and outline item is now packed into flated object
streams instead of being a bare indirect object. Same pages, same morning,
corpus bytes:

| Page | Links | Before | After | Chrome |
|---|---|---|---|---|
| Wikipedia (Go) | 710 | 482 KB | **251 KB** | 1.7 MB |
| countries table | 876 | 368 KB | **126 KB** | 1.7 MB |
| pkg.go.dev | 1790 | 735 KB | **384 KB** | 6.3 MB |
| RFC 9110 | 3398 | 1.80 MB | **1.06 MB** | 4.5 MB |
| Hacker News | 228 | 72 KB | **28 KB** | 358 KB |
| go.dev/blog | 31 | 71 KB | 61 KB | 225 KB |
| react.dev | 158 | 7.87 MB | 7.83 MB | 2.7 MB |

The #14 residual ("RFC 9110 1.0 → 2.2 MB for its annotations") is closed:
the RFC is back at 1.06 MB with every link, and every reader on the machine
reads the PDF 1.5 files with no warning (JUDGES.md, rerun on these files).
Nothing else in the file changed — page count, text, link count and the
judges' consensus are identical between the two stages.

**The one page where we are now larger than Chrome, and why.** react.dev is
7.8 MB against Chrome's 2.7 MB. `pdfimages -list` says where it goes: its
stylesheet lays out the eight conference photographs the site serves as
**WebP**, which the engine decodes and we embed as flate RGB bitmaps at the
1024 px they were fetched at — 0.5 to 1.3 MB each, ~7 MB of the file,
deduplicated (8 objects for 16 uses) but painted in a grid ~300 px wide.
Two levers, both outside this change: embed a bitmap at its *painted* size
(times a print scale), not its fetched size; and pass a JPEG source through
as a DCTDecode stream instead of re-encoding it. The same explains the RSS
rise on that page.

**A counting trap this stage found**: the Links column used to count
`/Subtype /Link` in the raw bytes, which object streams hide; the counter
now inflates every flate stream first (`corpus/internal/pdfstat`) — and its
first version lost exactly one object stream of 1 000 annotations because
its regexp ate a data byte before `endstream` when the flate data ended in
0x0D. The number that exposed it was pdf.js's `getAnnotations` (3398), not
anything in the file. A count of the PDF's bytes is not a reader's count.
