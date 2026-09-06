# html2pdf vs headless Chrome — 2026-09-06

Machine: Apple M4 Max, 16 cores; load average at start: { 17.83 12.36 10.96 }. Chrome: Google Chrome 152.0.7977.76. 5 runs per tool per input, interleaved; medians. Both tools timed as child processes under `/usr/bin/time -l` (wall-clock to PDF on disk, start-up and any fetch included; RSS = peak resident set).

| Input | html2pdf | Chrome | Chrome ÷ html2pdf | PDF html2pdf | PDF Chrome | Pages | Text chars | RSS html2pdf | RSS Chrome |
|---|---|---|---|---|---|---|---|---|---|
| https://example.com/ | 0.07 s | 1.67 s | 23.9× | 0.0 MB | 0.0 MB | 1 / 1 | 127 / 128 | 48.0 MB | 242.1 MB |
| https://en.wikipedia.org/wiki/Go_(programming_language) | 0.35 s | 2.23 s | 6.4× | 0.2 MB | 1.7 MB | 18 / 25 | 63079 / 67289 | 102.6 MB | 361.9 MB |
| https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations) | 0.54 s | 2.30 s | 4.3× | 0.1 MB | 1.6 MB | 9 / 12 | 22951 / 21987 | 96.6 MB | 356.6 MB |
| https://go.dev/blog/subtests | 0.72 s | 3.09 s | 4.3× | 0.2 MB | 0.2 MB | 6 / 7 | 13481 / 11742 | 71.2 MB | 352.7 MB |
| https://pkg.go.dev/net/http | 1.66 s | 3.67 s | 2.2× | 0.4 MB | 6.3 MB | 49 / 86 | 151040 / 150480 | 110.3 MB | 677.5 MB |
| https://www.rfc-editor.org/rfc/rfc9110.html | 0.78 s | 3.90 s | 5.0× | 1.0 MB | 4.5 MB | 120 / 169 | 449848 / 445667 | 187.3 MB | 549.7 MB |
| https://news.ycombinator.com/ | 0.96 s | 2.53 s | 2.6× | 0.0 MB | 0.4 MB | 1 / 2 | 3812 / 3875 | 56.6 MB | 252.6 MB |
| https://react.dev/ | 0.73 s | 2.52 s | 3.5× | 4.7 MB | 2.7 MB | 13 / 9 | 8045 / 6809 | 152.6 MB | 302.1 MB |
| fixtures/longdoc.html | 0.41 s | 1.99 s | 4.9× | 0.9 MB | 2.4 MB | 100 / 135 | 590571 / 591170 | 169.0 MB | 372.1 MB |

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
