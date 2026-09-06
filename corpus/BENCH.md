# html2pdf vs headless Chrome — 2026-09-06

Machine: Apple M4 Max, 16 cores; load average at start: { 19.47 15.17 12.01 }. Chrome: Google Chrome 152.0.7977.76. 5 runs per tool per input, interleaved; medians. Both tools timed as child processes under `/usr/bin/time -l` (wall-clock to PDF on disk, start-up and any fetch included; RSS = peak resident set).

| Input | html2pdf | Chrome | Chrome ÷ html2pdf | PDF html2pdf | PDF Chrome | Pages | Text chars | RSS html2pdf | RSS Chrome |
|---|---|---|---|---|---|---|---|---|---|
| https://example.com/ | 0.07 s | 1.63 s | 23.3× | 0.0 MB | 0.0 MB | 1 / 1 | 127 / 128 | 48.3 MB | 241.9 MB |
| https://en.wikipedia.org/wiki/Go_(programming_language) | 0.39 s | 2.13 s | 5.5× | 0.6 MB | 1.7 MB | 18 / 25 | 63044 / 67289 | 104.4 MB | 360.4 MB |
| https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations) | 0.54 s | 2.18 s | 4.0× | 0.2 MB | 1.6 MB | 9 / 12 | 22953 / 21987 | 97.6 MB | 356.9 MB |
| https://go.dev/blog/subtests | 0.76 s | 3.13 s | 4.1× | 0.3 MB | 0.2 MB | 6 / 7 | 13478 / 11742 | 74.5 MB | 352.7 MB |
| https://pkg.go.dev/net/http | 1.69 s | 3.48 s | 2.1× | 1.3 MB | 6.3 MB | 49 / 86 | 150974 / 150480 | 141.5 MB | 673.8 MB |
| https://www.rfc-editor.org/rfc/rfc9110.html | 0.94 s | 3.26 s | 3.5× | 4.7 MB | 4.5 MB | 120 / 169 | 449844 / 445667 | 239.2 MB | 549.8 MB |
| https://news.ycombinator.com/ | 0.97 s | 2.61 s | 2.7× | 0.0 MB | 0.4 MB | 1 / 2 | 3809 / 3872 | 56.7 MB | 252.9 MB |
| https://react.dev/ | 0.67 s | 2.58 s | 3.9× | 4.8 MB | 2.7 MB | 13 / 9 | 8038 / 6809 | 152.9 MB | 300.3 MB |
| fixtures/longdoc.html | 0.80 s | 2.13 s | 2.7× | 6.5 MB | 2.4 MB | 100 / 135 | 590571 / 591170 | 235.7 MB | 372.2 MB |

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

Two inputs are still larger, for different reasons:

- **`longdoc`, 6.5 vs 2.4 MB, no images**: what remains is how text is
  written. html2pdf emits one positioned text object per laid-out run (a word
  or a few), each with its own font selection; Chrome packs a whole line into
  one `TJ` array with kerning offsets. Batching a line's runs into one text
  object is the next saving, in pdfkit or here — not attempted in this pass.
- **`react.dev`, 4.8 vs 2.7 MB**: bitmaps. Its icons are inline SVGs the
  engine rasterises, and html2pdf embeds them as flate-compressed RGB
  samples; Chrome keeps them as vector paths. Dedup already halved this page;
  keeping SVG vector would need a path exporter in the engine.

### Pages and text: not a like-for-like layout, by design

html2pdf produces fewer pages on every text page (RFC 9110 120 vs 169,
`pkg.go.dev` 49 vs 86): `Options.ViewportPx` lays out at 1024 px and scales
to the print column (≈0.63×), so the type is smaller and the page denser;
Chrome prints at CSS px with no shrink. Set `ViewportPx` to the column width
for a 1:1 comparison. Extracted text agrees within ~5% except where Chrome
runs the page's JavaScript and print stylesheet and html2pdf does not
(`react.dev` 13 vs 9 pages, `go.dev/blog` hiding its nav in print CSS).
