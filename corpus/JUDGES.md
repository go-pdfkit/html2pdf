# html2pdf output judged by every reader on this machine — 2026-09-06

Judges: qpdf, poppler, mupdf, gs, pdfium, pdfjs, quartz. Versions: poppler pdftoppm version 26.04.0; mupdf mutool version 1.28.3; gs 10.07.1; qpdf qpdf version 12.4.1; pdf.js 6.3.289; macOS 26.6.2.

Cell format: `pages · text ratio · Δrender` — pages the judge reports (– if it reports none), its extracted-text length as a ratio of poppler's (– if it extracts none), and its page-1 render's distance from poppler's (mean grey Δ on 0–255 / share of pixels off by more than 48). ⚠n = n warning lines on stderr; ❌ = the judge failed to process the file.

| PDF | Bytes | qpdf | poppler | mupdf | gs | pdfium | pdfjs | quartz |
|---|---|---|---|---|---|---|---|---|
| out/bench/en-wikipedia-org-wiki-Go_programming_language.chrome.pdf | 1.7 MB | ✅ – · – · – | ✅ 25p · 1.00 · ref | ✅ 25p · 1.00 · Δ7.0/5.4% | ✅ – · 1.00 · Δ10.9/9.1% | ✅ 25p · 1.00 · Δ6.8/5.2% | ✅ 25p · 1.00 · Δ6.5/4.1% | ✅ – · – · Δ9.3/7.5% |
| out/bench/en-wikipedia-org-wiki-Go_programming_language.html2pdf.pdf | 208 KB | ✅ – · – · – | ✅ 18p · 1.00 · ref | ✅ 18p · 1.00 · Δ0.4/0.1% | ✅ – · 1.00 · Δ1.7/1.5% | ✅ 18p · 1.00 · Δ1.0/0.8% | ✅ 18p · 1.00 · Δ0.5/0.1% | ✅ – · – · Δ1.2/1.1% |
| out/bench/en-wikipedia-org-wiki-List_of_countries_by_population_United_Nations.chrome.pdf | 1.6 MB | ✅ – · – · – | ✅ 12p · 1.00 · ref | ✅ 12p · 1.00 · Δ2.2/1.3% | ✅ – · 1.01 · Δ4.1/3.3% | ✅ 12p · 1.00 · Δ2.4/1.7% | ✅ 12p · 1.00 · Δ2.1/1.0% | ✅ – · – · Δ2.9/2.0% |
| out/bench/en-wikipedia-org-wiki-List_of_countries_by_population_United_Nations.html2pdf.pdf | 81 KB | ✅ – · – · – | ✅ 9p · 1.00 · ref | ✅ 9p · 1.00 · Δ0.4/0.1% | ✅ – · 1.00 · Δ1.6/1.4% | ✅ 9p · 1.00 · Δ0.9/0.7% | ✅ 9p · 1.00 · Δ0.5/0.1% | ✅ – · – · Δ1.2/1.0% |
| out/bench/example-com.chrome.pdf | 35 KB | ✅ – · – · – | ✅ 1p · 1.00 · ref | ✅ 1p · 1.00 · Δ1.2/0.3% | ✅ – · 1.00 · Δ1.0/0.8% | ✅ 1p · 1.00 · Δ0.4/0.2% | ✅ 1p · 1.00 · Δ0.4/0.3% | ✅ – · – · Δ0.4/0.2% |
| out/bench/example-com.html2pdf.pdf | 8 KB | ✅ – · – · – | ✅ 1p · 1.00 · ref | ✅ 1p · 1.00 · Δ0.1/0.0% | ✅ – · 1.00 · Δ0.4/0.3% | ✅ 1p · 1.00 · Δ0.2/0.1% | ✅ 1p · 1.00 · Δ0.1/0.0% | ✅ – · – · Δ0.3/0.2% |
| out/bench/fixtures-longdoc-html.chrome.pdf | 2.4 MB | ✅ – · – · – | ✅ 135p · 1.00 · ref | ✅ 135p · 1.00 · Δ4.8/2.3% | ✅ – · 0.99 · Δ8.7/6.6% | ✅ 135p · 1.00 · Δ5.3/2.9% | ✅ 135p · 1.00 · Δ4.5/1.3% | ✅ – · – · Δ7.7/4.9% |
| out/bench/fixtures-longdoc-html.html2pdf.pdf | 873 KB | ✅ – · – · – | ✅ 100p · 1.00 · ref | ✅ 100p · 1.00 · Δ4.7/1.3% | ✅ – · 1.00 · Δ16.5/15.2% | ✅ 100p · 1.00 · Δ11.3/9.2% | ✅ 100p · 1.00 · Δ4.5/1.3% | ✅ – · – · Δ13.3/11.9% |
| out/bench/go-dev-blog-subtests.chrome.pdf | 225 KB | ✅ – · – · – | ✅ 7p · 1.00 · ref | ✅ 7p · 1.00 · Δ4.7/3.4% | ✅ – · 1.00 · Δ9.0/8.0% | ✅ 7p · 1.00 · Δ4.0/2.9% | ✅ 7p · 1.00 · Δ4.0/2.8% | ✅ – · – · Δ4.3/3.1% |
| out/bench/go-dev-blog-subtests.html2pdf.pdf | 181 KB | ✅ – · – · – | ✅ 6p · 1.00 · ref | ✅ 6p · 1.00 · Δ0.7/0.1% | ✅ – · 1.00 · Δ3.2/2.8% | ✅ 6p · 1.00 · Δ1.9/1.5% | ✅ 6p · 1.00 · Δ0.7/0.1% | ✅ – · – · Δ2.4/2.0% |
| out/bench/news-ycombinator-com.chrome.pdf | 358 KB | ✅ – · – · – | ✅ 2p · 1.00 · ref | ✅ 2p · 1.00 · Δ3.2/1.2% | ✅ – · 1.00 · Δ5.7/3.8% | ✅ 2p · 1.00 · Δ2.6/1.2% | ✅ 2p · 1.00 · Δ2.6/0.7% | ✅ – · – · Δ4.9/2.8% |
| out/bench/news-ycombinator-com.html2pdf.pdf | 22 KB | ✅ – · – · – | ✅ 1p · 1.00 · ref | ✅ 1p · 1.00 · Δ3.3/0.5% | ✅ – · 1.00 · Δ11.1/10.6% | ✅ 1p · 1.00 · Δ6.6/5.1% | ✅ 1p · 1.00 · Δ2.9/0.4% | ✅ – · – · Δ8.1/6.6% |
| out/bench/pkg-go-dev-net-http.chrome.pdf | 6.3 MB | ✅ – · – · – | ✅ 86p · 1.00 · ref | ✅ 86p · 1.00 · Δ2.7/1.5% | ✅ – · 1.00 · Δ5.6/4.7% | ✅ 86p · 1.00 · Δ2.2/1.3% | ✅ 86p · 1.00 · Δ2.5/1.4% | ✅ – · – · Δ3.2/1.5% |
| out/bench/pkg-go-dev-net-http.html2pdf.pdf | 386 KB | ✅ – · – · – | ✅ 49p · 1.00 · ref | ✅ 49p · 1.00 · Δ0.6/0.1% | ✅ – · 1.00 · Δ2.8/2.5% | ✅ 49p · 1.00 · Δ1.7/1.4% | ✅ 49p · 1.00 · Δ0.7/0.2% | ✅ – · – · Δ2.2/1.9% |
| out/bench/react-dev.chrome.pdf | 2.7 MB | ✅ – · – · – | ✅ 9p · 1.00 · ref | ✅ 9p · 1.00 · Δ0.2/0.1% | ✅ – · 1.00 · Δ0.5/0.4% | ✅ 9p · 1.00 · Δ0.2/0.1% | ✅ 9p · 1.00 · Δ0.2/0.1% | ✅ – · – · Δ0.4/0.3% |
| out/bench/react-dev.html2pdf.pdf | 4.7 MB | ✅ – · – · – | ✅ 13p · 1.00 · ref | ✅ 13p · 1.00 · Δ1.2/0.4% | ✅ – · 1.00 · Δ4.7/4.0% | ✅ 13p · 1.00 · Δ3.3/2.9% | ✅ 13p · 1.00 · Δ1.3/0.5% | ✅ – · – · Δ4.0/3.5% |
| out/bench/www-rfc-editor-org-rfc-rfc9110-html.chrome.pdf | 4.5 MB | ✅ – · – · – | ✅ 169p · 1.00 · ref | ✅ 169p · 1.00 · Δ3.2/2.3% | ✅ – · 1.00 · Δ5.2/4.5% | ✅ 169p · 1.00 · Δ4.0/3.0% | ✅ 169p · 1.00 · Δ2.9/1.4% | ✅ – · – · Δ4.6/3.7% |
| out/bench/www-rfc-editor-org-rfc-rfc9110-html.html2pdf.pdf | 982 KB | ✅ – · – · – | ✅ 120p · 1.00 · ref | ✅ 120p · 1.00 · Δ3.1/1.0% | ✅ – · 1.00 · Δ10.6/9.9% | ✅ 120p · 1.00 · Δ7.4/6.3% | ✅ 120p · 1.00 · Δ2.9/0.8% | ✅ – · – · Δ8.5/7.7% |
| out/en-wikipedia-org-wiki-Go_programming_language.pdf | 216 KB | ✅ – · – · – | ✅ 18p · 1.00 · ref | ✅ 18p · 1.00 · Δ0.4/0.1% | ✅ – · 1.00 · Δ1.7/1.5% | ✅ 18p · 1.00 · Δ1.0/0.8% | ✅ 18p · 1.00 · Δ0.5/0.1% | ✅ – · – · Δ1.2/1.1% |
| out/en-wikipedia-org-wiki-List_of_countries_by_population_United_Nations.pdf | 78 KB | ✅ – · – · – | ✅ 9p · 1.00 · ref | ✅ 9p · 1.00 · Δ0.4/0.1% | ✅ – · 1.00 · Δ1.6/1.4% | ✅ 9p · 1.00 · Δ0.9/0.7% | ✅ 9p · 1.00 · Δ0.5/0.1% | ✅ – · – · Δ1.2/1.0% |
| out/example-com.pdf | 8 KB | ✅ – · – · – | ✅ 1p · 1.00 · ref | ✅ 1p · 1.00 · Δ0.1/0.0% | ✅ – · 1.00 · Δ0.4/0.3% | ✅ 1p · 1.00 · Δ0.2/0.1% | ✅ 1p · 1.00 · Δ0.1/0.0% | ✅ – · – · Δ0.3/0.2% |
| out/go-dev-blog-subtests.pdf | 181 KB | ✅ – · – · – | ✅ 6p · 1.00 · ref | ✅ 6p · 1.00 · Δ0.7/0.1% | ✅ – · 1.00 · Δ3.2/2.8% | ✅ 6p · 1.00 · Δ1.9/1.5% | ✅ 6p · 1.00 · Δ0.7/0.1% | ✅ – · – · Δ2.4/2.0% |
| out/news-ycombinator-com.pdf | 22 KB | ✅ – · – · – | ✅ 1p · 1.00 · ref | ✅ 1p · 1.00 · Δ3.3/0.5% | ✅ – · 1.00 · Δ11.1/10.6% | ✅ 1p · 1.00 · Δ6.6/5.1% | ✅ 1p · 1.00 · Δ2.9/0.4% | ✅ – · – · Δ8.1/6.6% |
| out/pkg-go-dev-net-http.pdf | 386 KB | ✅ – · – · – | ✅ 49p · 1.00 · ref | ✅ 49p · 1.00 · Δ0.6/0.1% | ✅ – · 1.00 · Δ2.8/2.5% | ✅ 49p · 1.00 · Δ1.7/1.4% | ✅ 49p · 1.00 · Δ0.7/0.2% | ✅ – · – · Δ2.2/1.9% |
| out/react-dev.pdf | 4.7 MB | ✅ – · – · – | ✅ 13p · 1.00 · ref | ✅ 13p · 1.00 · Δ1.2/0.4% | ✅ – · 1.00 · Δ4.7/4.0% | ✅ 13p · 1.00 · Δ3.3/2.9% | ✅ 13p · 1.00 · Δ1.3/0.5% | ✅ – · – · Δ4.0/3.5% |
| out/www-rfc-editor-org-rfc-rfc9110-html.pdf | 982 KB | ✅ – · – · – | ✅ 120p · 1.00 · ref | ✅ 120p · 1.00 · Δ3.1/1.0% | ✅ – · 1.00 · Δ10.6/9.9% | ✅ 120p · 1.00 · Δ7.4/6.3% | ✅ 120p · 1.00 · Δ2.9/0.8% | ✅ – · – · Δ8.5/7.7% |

<!-- BEGIN ANALYSIS -->

## Analysis — 2026-09-06

**26 PDFs × 7 judges: no judge failed to open, paginate, render or extract
any file, and none printed a warning.** `qpdf --check` is clean on every
file. That is the first-order answer to "is the smallest file also read
correctly by the rest of the field": yes, by every reader on this machine —
poppler, MuPDF, Ghostscript, pdfium (Chrome's engine), pdf.js (Firefox's),
Quartz (Preview's) — with Chrome's own PDFs of the same pages judged
alongside as the control.

### Text: the five extractors agree on html2pdf's files to within 3 characters

Non-whitespace characters recovered from the same file by poppler / MuPDF /
Ghostscript / pdfium / pdf.js:

| PDF (html2pdf) | poppler | MuPDF | gs | pdfium | pdf.js |
|---|---|---|---|---|---|
| RFC 9110 | 378,290 | 378,293 | 378,293 | 378,293 | 378,293 |
| `longdoc` | 516,246 | 516,246 | 516,246 | 516,246 | 516,246 |
| `pkg.go.dev/net/http` | 125,449 | 125,449 | 125,451 | 125,451 | 125,449 |
| Wikipedia (Go) | 52,778 | 52,778 | 52,778 | 52,778 | 52,778 |
| Hacker News | 3,153 | 3,153 | 3,153 | 3,153 | 3,153 |

The same five judges spread more on **Chrome's** files: Wikipedia
58,270–58,302, RFC 9110 376,539–377,127, and Ghostscript loses 4,899
characters of Chrome's `longdoc` (511,347 vs 516,246) that every other judge
finds. Every judge that reports a page count agrees with poppler on every
file, ours and Chrome's. So the one-`TJ`-per-run text (pdfkit #28) and the
compressed streams that made these files small are read identically by all
five parsers — the encoding change cost nothing in interoperability.

### Renders: MuPDF and pdf.js within 1.3% of poppler; gs/pdfium/Quartz differ more on dense pages, and it is anti-aliasing

Page-1 render distance from poppler's (share of pixels off by >48/255):
MuPDF ≤1.3% and pdf.js ≤0.8% on every html2pdf file. Ghostscript, pdfium and
Quartz run higher on the densest pages — `longdoc` 15.2% / 9.2% / 11.9%, RFC
9110 9.9% / 6.3% / 7.7%, Hacker News 10.6% / 5.1% / 6.6% — and higher than
on Chrome's versions of the same pages. Checked by eye
(`out/judges/fixtures-longdoc-html.html2pdf/{gs,poppler}.png`): the two
renders have the same line breaks, the same badge, table and code-block
positions — Ghostscript simply draws the glyphs heavier at 72 dpi. html2pdf
lays out at 1024 px and scales to the print column (0.63×), so its body text
is ~8 pt and a page carries far more glyph edges than Chrome's 1:1 page;
each rasteriser's own anti-aliasing then disagrees on more pixels. That is
rasteriser noise scaling with edge density, not displacement.

### What the harness had to get right to say any of this

Three judge-side traps, each of which would have read as an html2pdf defect:
Quartz (`sips`) renders on a transparent background, and a transparent
pixel converted straight to grey is black — every Quartz render scored 99%
different until renders were composited over white first; `pdfium_test
--txt` writes UTF-32LE (BOM `FF FE 00 00`, verified with `xxd`), so counting
bytes read as four times the text; and Ghostscript's `txtwrite` pads lines
with spaces to reproduce columns while pdfium separates every glyph run, so
text is compared as non-whitespace characters. pdfium's "⚠4" in an earlier
pass was its two progress lines per invocation, now excluded.

### Limits

Renders are compared on page 1 only, at 72 dpi, against poppler as the
reference — a disagreement shared by all judges would be invisible here, and
a page-N defect would be missed; text and page counts cover every page.
Firefox and Chrome themselves do not render a PDF in headless mode, so their
engines are judged through pdf.js-in-node and `pdfium_test`. Adobe Reader is
not on this machine. ABBYY FineReader is installed but not scriptable here.
