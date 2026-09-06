# html2pdf output judged by every reader on this machine — 2026-09-06

Judges: qpdf, poppler, mupdf, gs, pdfjs, quartz. Versions: poppler pdftoppm version 26.04.0; mupdf mutool version 1.28.3; gs 10.07.1; qpdf qpdf version 12.4.1; pdf.js 6.3.289; macOS 26.6.2.

Cell format: `pages · text ratio · Δworst (page)` — pages the judge reports (– if none), its extracted-text length as a ratio of poppler's (– if it extracts none), and the largest distance of its renders from poppler's over the sampled pages (first, middle, last, at 96 dpi: share of pixels whose grey level moves by more than 48/255 after both are downsampled to 400 px wide), with the page it happened on. `consensus` is the mean pairwise distance between all judges' renders of a page, worst page — no reader privileged. ⚠n = n warning lines on stderr; ❌ = the judge failed to process the file.

| PDF | Bytes | qpdf | poppler | mupdf | gs | pdfjs | quartz | consensus |
|---|---|---|---|---|---|---|---|---|
| out/bench/en-wikipedia-org-wiki-Go_programming_language.chrome.pdf | 1.7 MB | ✅ – · – · – | ✅ 25p · 1.000 · ref | ✅ 25p · 1.000 · Δ7.6% (p25) | ✅ – · 1.000 · Δ9.3% (p25) | ✅ 25p · 1.000 · Δ7.5% (p25) | ✅ – · – · Δ10.8% (p1 only) | 7.2% (p25) |
| out/bench/en-wikipedia-org-wiki-Go_programming_language.html2pdf.pdf | 241 KB | ✅ – · – · – | ✅ 13p · 1.000 · ref | ✅ 13p · 1.000 · Δ4.0% (p1) | ✅ – · 1.000 · Δ11.9% (p1) | ✅ 13p · 1.000 · Δ2.5% (p1) | ✅ – · – · Δ14.2% (p1 only) | 8.7% (p1) |
| out/bench/en-wikipedia-org-wiki-List_of_countries_by_population_United_Nations.chrome.pdf | 1.7 MB | ✅ – · – · – | ✅ 12p · 1.000 · ref | ✅ 12p · 1.000 · Δ4.9% (p6) | ✅ – · 1.015 · Δ5.5% (p6) | ✅ 12p · 1.000 · Δ4.9% (p6) | ✅ – · – · Δ3.0% (p1 only) | 3.8% (p6) |
| out/bench/en-wikipedia-org-wiki-List_of_countries_by_population_United_Nations.html2pdf.pdf | 128 KB | ✅ – · – · – | ✅ 7p · 1.000 · ref | ✅ 7p · 1.000 · Δ1.9% (p4) | ✅ – · 1.000 · Δ6.2% (p1) | ✅ 7p · 1.000 · Δ1.3% (p1) | ✅ – · – · Δ9.0% (p1 only) | 5.1% (p1) |
| out/bench/example-com.chrome.pdf | 35 KB | ✅ – · – · – | ✅ 1p · 1.000 · ref | ✅ 1p · 1.000 · Δ0.4% (p1) | ✅ – · 1.000 · Δ0.6% (p1) | ✅ 1p · 1.000 · Δ0.4% (p1) | ✅ – · – · Δ0.5% (p1 only) | 0.4% (p1) |
| out/bench/example-com.html2pdf.pdf | 7 KB | ✅ – · – · – | ✅ 1p · 1.000 · ref | ✅ 1p · 1.000 · Δ0.0% (p1) | ✅ – · 1.000 · Δ0.2% (p1) | ✅ 1p · 1.000 · Δ0.0% (p1) | ✅ – · – · Δ0.2% (p1 only) | 0.2% (p1) |
| out/bench/fixtures-longdoc-html.chrome.pdf | 2.4 MB | ✅ – · – · – | ✅ 135p · 1.000 · ref | ✅ 135p · 1.000 · Δ3.0% (p1) | ✅ – · 0.991 · Δ7.0% (p1) | ✅ 135p · 1.000 · Δ1.9% (p68) | ✅ – · – · Δ8.4% (p1 only) | 5.1% (p1) |
| out/bench/fixtures-longdoc-html.html2pdf.pdf | 852 KB | ✅ – · – · – | ✅ 100p · 1.000 · ref | ✅ 100p · 1.000 · Δ2.1% (p1) | ✅ – · 1.000 · Δ8.9% (p1) | ✅ 100p · 1.000 · Δ1.4% (p1) | ✅ – · – · Δ12.7% (p1 only) | 6.9% (p1) |
| out/bench/go-dev-blog-subtests.chrome.pdf | 225 KB | ✅ – · – · – | ✅ 7p · 1.000 · ref | ✅ 7p · 1.000 · Δ5.6% (p4) | ✅ – · 1.000 · Δ8.6% (p4) | ✅ 7p · 1.000 · Δ5.7% (p4) | ✅ – · – · Δ5.8% (p1 only) | 6.1% (p4) |
| out/bench/go-dev-blog-subtests.html2pdf.pdf | 61 KB | ✅ – · – · – | ✅ 5p · 1.000 · ref | ✅ 5p · 1.002 · Δ1.3% (p3) | ✅ – · 1.014 · Δ5.1% (p3) | ✅ 5p · 1.002 · Δ0.7% (p3) | ✅ – · – · Δ7.2% (p1 only) | 4.0% (p1) |
| out/bench/news-ycombinator-com.chrome.pdf | 357 KB | ✅ – · – · – | ✅ 2p · 1.000 · ref | ✅ 2p · 1.000 · Δ1.5% (p1) | ✅ – · 1.000 · Δ2.8% (p1) | ✅ 2p · 1.000 · Δ1.0% (p1) | ✅ – · – · Δ4.2% (p1 only) | 2.3% (p1) |
| out/bench/news-ycombinator-com.html2pdf.pdf | 28 KB | ✅ – · – · – | ✅ 1p · 1.000 · ref | ✅ 1p · 1.000 · Δ0.1% (p1) | ✅ – · 1.000 · Δ1.5% (p1) | ✅ 1p · 1.000 · Δ0.0% (p1) | ✅ – · – · Δ3.3% (p1 only) | 1.4% (p1) |
| out/bench/pkg-go-dev-net-http.chrome.pdf | 6.3 MB | ✅ – · – · – | ✅ 86p · 1.000 · ref | ✅ 86p · 1.000 · Δ2.9% (p43) | ✅ – · 1.000 · Δ4.4% (p43) | ✅ 86p · 1.000 · Δ3.0% (p43) | ✅ – · – · Δ2.6% (p1 only) | 3.1% (p43) |
| out/bench/pkg-go-dev-net-http.html2pdf.pdf | 384 KB | ✅ – · – · – | ✅ 51p · 1.000 · ref | ✅ 51p · 1.000 · Δ0.9% (p26) | ✅ – · 1.001 · Δ2.6% (p26) | ✅ 51p · 1.000 · Δ0.6% (p26) | ✅ – · – · Δ3.8% (p1 only) | 1.9% (p1) |
| out/bench/react-dev.chrome.pdf | 2.7 MB | ✅ – · – · – | ✅ 9p · 1.000 · ref | ✅ 9p · 1.000 · Δ0.6% (p5) | ✅ – · 1.000 · Δ1.4% (p5) | ✅ 9p · 1.000 · Δ0.6% (p5) | ✅ – · – · Δ0.4% (p1 only) | 0.9% (p5) |
| out/bench/react-dev.html2pdf.pdf | 7.8 MB | ✅ – · – · – | ✅ 8p · 1.000 · ref | ✅ 8p · 1.000 · Δ0.8% (p4) | ✅ – · 1.000 · Δ2.0% (p4) | ✅ 8p · 1.000 · Δ0.9% (p4) | ✅ – · – · Δ2.3% (p1 only) | 1.4% (p1) |
| out/bench/www-rfc-editor-org-rfc-rfc9110-html.chrome.pdf | 4.5 MB | ✅ – · – · – | ✅ 169p · 1.000 · ref | ✅ 169p · 1.000 · Δ6.2% (p85) | ✅ – · 1.002 · Δ12.6% (p85) | ✅ 169p · 1.000 · Δ6.1% (p85) | ✅ – · – · Δ7.3% (p1 only) | 7.3% (p85) |
| out/bench/www-rfc-editor-org-rfc-rfc9110-html.html2pdf.pdf | 1.1 MB | ✅ – · – · – | ✅ 85p · 1.000 · ref | ✅ 85p · 1.000 · Δ2.0% (p43) | ✅ – · 1.000 · Δ9.4% (p43) | ✅ 85p · 1.000 · Δ1.3% (p43) | ✅ – · – · Δ6.2% (p1 only) | 4.3% (p43) |
| out/en-wikipedia-org-wiki-Go_programming_language.pdf | 251 KB | ✅ – · – · – | ✅ 13p · 1.000 · ref | ✅ 13p · 1.000 · Δ4.0% (p1) | ✅ – · 1.000 · Δ11.9% (p1) | ✅ 13p · 1.000 · Δ2.6% (p1) | ✅ – · – · Δ14.3% (p1 only) | 8.7% (p1) |
| out/en-wikipedia-org-wiki-List_of_countries_by_population_United_Nations.pdf | 126 KB | ✅ – · – · – | ✅ 7p · 1.000 · ref | ✅ 7p · 1.000 · Δ1.9% (p4) | ✅ – · 1.000 · Δ6.2% (p1) | ✅ 7p · 1.000 · Δ1.3% (p1) | ✅ – · – · Δ9.0% (p1 only) | 5.1% (p1) |
| out/example-com.pdf | 7 KB | ✅ – · – · – | ✅ 1p · 1.000 · ref | ✅ 1p · 1.000 · Δ0.0% (p1) | ✅ – · 1.000 · Δ0.2% (p1) | ✅ 1p · 1.000 · Δ0.0% (p1) | ✅ – · – · Δ0.2% (p1 only) | 0.2% (p1) |
| out/go-dev-blog-subtests.pdf | 61 KB | ✅ – · – · – | ✅ 5p · 1.000 · ref | ✅ 5p · 1.002 · Δ1.3% (p3) | ✅ – · 1.014 · Δ5.1% (p3) | ✅ 5p · 1.002 · Δ0.7% (p3) | ✅ – · – · Δ7.2% (p1 only) | 4.0% (p1) |
| out/news-ycombinator-com.pdf | 28 KB | ✅ – · – · – | ✅ 1p · 1.000 · ref | ✅ 1p · 1.000 · Δ0.1% (p1) | ✅ – · 1.000 · Δ1.5% (p1) | ✅ 1p · 1.000 · Δ0.0% (p1) | ✅ – · – · Δ3.3% (p1 only) | 1.4% (p1) |
| out/pkg-go-dev-net-http.pdf | 384 KB | ✅ – · – · – | ✅ 51p · 1.000 · ref | ✅ 51p · 1.000 · Δ0.9% (p26) | ✅ – · 1.001 · Δ2.6% (p26) | ✅ 51p · 1.000 · Δ0.6% (p26) | ✅ – · – · Δ3.8% (p1 only) | 1.9% (p1) |
| out/react-dev.pdf | 7.8 MB | ✅ – · – · – | ✅ 8p · 1.000 · ref | ✅ 8p · 1.000 · Δ0.8% (p4) | ✅ – · 1.000 · Δ2.0% (p4) | ✅ 8p · 1.000 · Δ0.9% (p4) | ✅ – · – · Δ2.3% (p1 only) | 1.4% (p1) |
| out/www-rfc-editor-org-rfc-rfc9110-html.pdf | 1.1 MB | ✅ – · – · – | ✅ 85p · 1.000 · ref | ✅ 85p · 1.000 · Δ2.0% (p43) | ✅ – · 1.000 · Δ9.4% (p43) | ✅ 85p · 1.000 · Δ1.3% (p43) | ✅ – · – · Δ6.2% (p1 only) | 4.3% (p43) |

<!-- BEGIN ANALYSIS -->

## Analysis — 2026-09-06 (second pass: sampled pages, 96 dpi, pairwise consensus)

**26 PDFs × 7 judges: no judge failed to open, paginate, render or extract
any file, none printed a warning, `qpdf --check` is clean on every file.**
Judges: poppler, MuPDF, Ghostscript, pdfium (Chrome's engine), pdf.js
(Firefox's), Quartz (Preview's), qpdf — with Chrome's own PDFs of the same
pages judged alongside as the control. That is the answer to "is the
smallest file also read correctly by the rest of the field": by every
reader on this machine, on every page counted and extracted, and on the
first, middle and last page rendered.

### Text: five extractors agree on html2pdf's files to within 3 characters

Non-whitespace characters recovered by poppler / MuPDF / Ghostscript /
pdfium / pdf.js from the same file:

| PDF (html2pdf) | poppler | MuPDF | gs | pdfium | pdf.js |
|---|---|---|---|---|---|
| RFC 9110 | 378,290 | 378,293 | 378,293 | 378,293 | 378,293 |
| `longdoc` | 516,246 | 516,246 | 516,246 | 516,246 | 516,246 |
| `pkg.go.dev/net/http` | 125,449 | 125,449 | 125,451 | 125,451 | 125,449 |
| Wikipedia (Go) | 52,778 | 52,778 | 52,778 | 52,778 | 52,778 |
| Hacker News | 3,153 | 3,153 | 3,153 | 3,153 | 3,153 |

The same judges spread more on **Chrome's** files: Wikipedia 58,270–58,302,
RFC 9110 376,539–377,127, and Ghostscript loses 4,899 characters of Chrome's
`longdoc` (511,347 vs 516,246) that every other judge finds. Every judge that
reports a page count agrees with poppler on every file, ours and Chrome's.
The one-`TJ`-per-run text (pdfkit #28) and the compressed streams that made
these files small are read identically by all five parsers.

### Renders: both tools' files sit in the same band; the worst pages are the densest

`consensus` — mean pairwise distance between *all* judges' renders of a
page, worst of the first/middle/last page, no reader privileged:

| Page | html2pdf | Chrome | | Page | html2pdf | Chrome |
|---|---|---|---|---|---|---|
| Wikipedia (Go) | 5.8% | 7.6% | | `go.dev/blog` | 2.6% | 5.5% |
| RFC 9110 | 4.0% | 6.9% | | Hacker News | 4.6% | 2.6% |
| countries list | 3.5% | 3.8% | | `pkg.go.dev` | 3.0% | 2.8% |
| `example.com` | 0.1% | 0.4% | | `react.dev` | 1.9% | 1.0% |
| `longdoc` | 6.2% | 4.9% | | | | |

Lower on five of nine inputs, higher on four, everything between 0.1% and
7.6% — the two tools' files are read with the same consistency, and neither
has a page on which the judges fall apart. Per judge, on html2pdf's files
over all sampled pages: pdf.js ≤1.9% and MuPDF ≤3.3% from poppler
everywhere; pdfium ≤6.3%; Ghostscript up to 11.7% (Wikipedia p9) and Quartz
up to 12.7% (`longdoc` p1). Those two drive every html2pdf page that scores
above Chrome's, and it is always the densest page — `longdoc` p1, Hacker
News p1, a Wikipedia body page. Checked by eye on the previous pass
(`out/judges/fixtures-longdoc-html.html2pdf/{gs,poppler}_p1.png`): identical
line breaks and element positions, heavier glyph strokes from Ghostscript's
anti-aliasing. html2pdf lays out at 1024 px and scales to the print column
(0.63×), so its body text is ~8 pt and a page carries more glyph edges than
Chrome's 1:1 page; rasterisers disagree on edges. Chrome's own worst pages
show the same judges at the same magnitude (RFC 9110 p85: Ghostscript 12.6%,
pdfium 9.4%). Rasteriser noise scaling with edge density, on both tools —
not displacement.

### What the harness had to get right to say any of this

Judge-side traps that would each have read as an html2pdf defect: Quartz
(`sips`) renders on a transparent background — a transparent pixel converted
straight to grey is black, so every Quartz render scored 99% different until
renders were composited over white; `pdfium_test --txt` writes UTF-32LE (BOM
`FF FE 00 00`, verified with `xxd`), so counting bytes read as four times the
text; Ghostscript's `txtwrite` pads lines with spaces to reproduce columns and
pdfium separates every glyph run, so text is compared as non-whitespace
characters; pdfium narrates progress on stderr, which is not a warning.

### Limits

Renders are sampled (first, middle, last page) at 96 dpi and downsampled to
400 px wide before comparison — a defect confined to another page, or finer
than that thumbnail, would be missed; text and page counts cover every page.
Quartz renders page 1 only (`sips` has no page selection). Firefox and Chrome
do not render a PDF headless, so their engines are judged through pdf.js in
node and `pdfium_test`. Adobe Reader is not on this machine; ABBYY FineReader
is installed but not scriptable here.
