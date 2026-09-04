# corpus — html2pdf validated against real public pages

A repeatable harness that fetches each URL in `urls.txt`, renders it with
`html2pdf`, and records whether it survived plus a few robustness signals —
page count, extracted-text length, timing. Same spirit as
[`engine-webengine/bench`](https://github.com/go-webengine/engine/tree/main/bench):
real pages surface real shapes a hand-written test never would.

## Why a separate module

`corpus/` is its own Go module with a local `replace github.com/go-pdfkit/html2pdf => ..`,
so `go build ./...` / `go test ./...` from the repo root never see it and the
library's own dependency graph stays small.

## Run

```sh
cd corpus
go run ./cmd/corpus -urls urls.txt
```

Requires `pdfinfo` and `pdftotext` (poppler) and `pdftoppm` on `PATH`.

Outputs (relative to the working directory):

- `results.json` — per-URL `{url, ok, pages, pdf_bytes, text_chars, fetch_ms, render_ms, error}`
- `CORPUS.md` — the run header + table. The hand-written analysis below the
  `BEGIN ANALYSIS` marker is **preserved** across re-runs; only the table above
  it is overwritten.
- `out/<slug>.pdf` and `out/<slug>-p1-*.png` — the rendered PDF and a PNG of
  its first page, for a quick visual check without opening every PDF by hand.

## Adding a URL

Append it to `urls.txt` with a one-line comment saying what real-world shape
it's meant to stress (a nested layout table, a sticky sidebar, a huge data
table, ...) — the same convention `bench/urls.txt` uses. Prefer genuinely
document-shaped public pages (articles, specs, references) over app shells;
this tool has no JavaScript, so a JS-heavy SPA will only ever show its static
skeleton — which is a legitimate thing to keep one fixture around to prove,
not a reason to avoid the corpus entirely.
