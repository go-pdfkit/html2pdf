# html2pdf corpus run — 2026-09-04

8/8 succeeded.

| URL | Status | Pages | PDF | Text chars | Fetch | Render |
|---|---|---|---|---|---|---|
| [https://example.com/](https://example.com/) | ✅ | 1 | 26285 B | 127 | 41ms | 31ms |
| [https://en.wikipedia.org/wiki/Go_(programming_language)](https://en.wikipedia.org/wiki/Go_(programming_language)) | ✅ | 34 | 3530766 B | 63005 | 135ms | 108ms |
| [https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)](https://en.wikipedia.org/wiki/List_of_countries_by_population_(United_Nations)) | ✅ | 17 | 1347476 B | 23256 | 49ms | 144ms |
| [https://go.dev/blog/subtests](https://go.dev/blog/subtests) | ✅ | 9 | 825623 B | 13334 | 165ms | 38ms |
| [https://pkg.go.dev/net/http](https://pkg.go.dev/net/http) | ✅ | 86 | 8258945 B | 150642 | 487ms | 104ms |
| [https://www.rfc-editor.org/rfc/rfc9110.html](https://www.rfc-editor.org/rfc/rfc9110.html) | ✅ | 428 | 28285153 B | 450968 | 207ms | 488ms |
| [https://news.ycombinator.com/](https://news.ycombinator.com/) | ✅ | 4 | 239893 B | 4022 | 469ms | 33ms |
| [https://react.dev/](https://react.dev/) | ✅ | 7 | 501695 B | 7909 | 55ms | 43ms |

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

### Real limitation, not a bug: narrow print column vs. desktop-only responsive CSS

`rfc-editor.org`'s RFC 9110 page renders technically correctly but
inefficiently: 428 pages for a document whose official PDF runs closer to
180. `out/www-rfc-editor-org-rfc-rfc9110-html-p1-001.png` shows why — the
page's table-of-contents sidebar sits *beside* the article in a fixed-width
column, and at html2pdf's 170mm (≈642px) print column that squeezes the
actual prose down to under half the page width, so it wraps into roughly
twice the line count it would at full desktop width. The page was designed
for a 1200px+ viewport with no narrower breakpoint that drops the sidebar;
html2pdf has no `@media print` handling or "render wide, shrink to fit" mode
to compensate. `pkg.go.dev/net/http` at 86 pages is plausibly just genuinely
long (net/http is one of the largest stdlib packages) rather than showing the
same artifact — not confirmed either way.

**Possible future direction**: lay out at a wider virtual viewport (matching
what the page's own CSS was designed for) and scale the result down to the
print column, the way a browser's print dialog often does — real work, not
attempted here.

### Confirmed-expected: `react.dev` shows only its static shell

7 pages of real content (nav, hero, first section, one code sample) — the
part of the page that ships in the HTML itself. No JavaScript runs, so
nothing that hydrates client-side appears. This is the documented scope
limitation working as expected, not a failure.
