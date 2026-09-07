// Command corpus fetches each URL in urls.txt, renders it with html2pdf, and
// records whether it survived, how big the result is, and how much text
// came out the other end — a repeatable signal for real-world robustness,
// in the spirit of engine-webengine's own bench/ harness.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/go-pdfkit/html2pdf"
	"github.com/go-pdfkit/html2pdf/corpus/internal/pdfstat"
	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
)

type result struct {
	URL        string `json:"url"`
	Slug       string `json:"slug"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	FetchMs    int64  `json:"fetch_ms"`
	RenderMs   int64  `json:"render_ms"`
	PDFBytes   int    `json:"pdf_bytes"`
	Pages      int    `json:"pages"`
	TextChars  int    `json:"text_chars"`
	HTMLBytes  int    `json:"html_bytes"`
	PdfInfoErr string `json:"pdfinfo_err,omitempty"`
	Links      int    `json:"links"` // link annotations written (external URI + in-document GoTo)
	// LostChars counts the distinct non-space characters of the page's text
	// (the DOM's text nodes) that the PDF's extracted text never contains —
	// a glyph the fonts could not draw, most often. Lost lists them.
	LostChars int    `json:"lost_chars"`
	Lost      string `json:"lost,omitempty"`
}

func slugify(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "page"
	}
	s := u.Host + u.Path
	s = strings.Trim(s, "/")
	repl := strings.NewReplacer("/", "-", ":", "-", "(", "", ")", "", ".", "-")
	s = repl.Replace(s)
	if s == "" {
		s = "root"
	}
	return s
}

func readURLs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

func pdfPageCount(path string) (int, error) {
	out, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			var n int
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")), "%d", &n)
			return n, nil
		}
	}
	return 0, fmt.Errorf("no Pages: line in pdfinfo output")
}

func pdfTextChars(path string) int {
	out, err := exec.Command("pdftotext", path, "-").Output()
	if err != nil {
		return -1
	}
	return len(strings.TrimSpace(string(out)))
}

// renderWithRecover isolates a panic in the layout/paint pipeline to one
// corpus entry instead of aborting the whole run — real-world pages are
// exactly where an unanticipated shape shows up first. baseURL lets a
// relative <img src> resolve, so image-bearing pages are exercised for real.
func renderWithRecover(html, baseURL string) (pdfBytes []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	doc, exportErr := html2pdf.Export(html, html2pdf.Options{BaseURL: baseURL})
	if exportErr != nil {
		return nil, exportErr
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func run() int {
	urlsPath := flag.String("urls", "urls.txt", "file of URLs, one per line")
	outDir := flag.String("out", "out", "directory for rendered PDFs and page-1 PNGs")
	resultsPath := flag.String("results", "results.json", "path to write the JSON results")
	reportPath := flag.String("report", "CORPUS.md", "path to write the Markdown report")
	timeout := flag.Duration("timeout", 45*time.Second, "per-URL fetch+render timeout")
	flag.Parse()

	urls, err := readURLs(*urlsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpus: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "corpus: %v\n", err)
		return 1
	}

	e := engine.New()
	var results []result
	for _, u := range urls {
		fmt.Fprintf(os.Stderr, "fetching %s\n", u)
		r := result{URL: u, Slug: slugify(u)}

		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		t0 := time.Now()
		doc, err := e.Fetch(ctx, u)
		r.FetchMs = time.Since(t0).Milliseconds()
		cancel()
		if err != nil {
			r.Error = "fetch: " + err.Error()
			results = append(results, r)
			continue
		}
		r.HTMLBytes = len(doc.HTML)

		t1 := time.Now()
		pdfBytes, err := renderWithRecover(doc.HTML, doc.URL)
		r.RenderMs = time.Since(t1).Milliseconds()
		if err != nil {
			r.Error = "render: " + err.Error()
			results = append(results, r)
			continue
		}
		r.PDFBytes = len(pdfBytes)
		r.Links = pdfstat.CountLinks(pdfBytes)

		outPath := filepath.Join(*outDir, r.Slug+".pdf")
		if err := os.WriteFile(outPath, pdfBytes, 0o644); err != nil {
			r.Error = "write: " + err.Error()
			results = append(results, r)
			continue
		}

		if pages, err := pdfPageCount(outPath); err != nil {
			r.PdfInfoErr = err.Error()
		} else {
			r.Pages = pages
		}
		r.TextChars = pdfTextChars(outPath)
		r.LostChars, r.Lost = lostChars(e, doc, outPath)
		r.OK = true

		pngBase := filepath.Join(*outDir, r.Slug+"-p1")
		exec.Command("pdftoppm", "-png", "-r", "100", "-f", "1", "-l", "1", outPath, pngBase).Run()

		results = append(results, r)
	}

	rf, err := os.Create(*resultsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpus: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(rf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		rf.Close()
		fmt.Fprintf(os.Stderr, "corpus: %v\n", err)
		return 1
	}
	rf.Close()

	if err := writeReport(*reportPath, results); err != nil {
		fmt.Fprintf(os.Stderr, "corpus: report: %v\n", err)
		return 1
	}
	return 0
}

func main() { os.Exit(run()) }

// lostChars compares the characters of the document's rendered text — text
// nodes under elements the print cascade displays — with the characters
// poppler extracts from the PDF, and returns how many distinct non-space
// characters the PDF never contains, with the list (a glyph census: a
// character the fonts have no glyph for is drawn as nothing, and no page
// count or text length notices).
func lostChars(e *engine.Engine, doc *engine.Document, pdf string) (int, string) {
	// Cascade as Export does — the print medium, the page's own external
	// stylesheets — so what a site hides in print (Wikipedia's language
	// list, a nav) is not counted as lost.
	media := css.Media{Type: css.Print, Width: 1024}
	sheets := e.LoadStylesheets(context.Background(), doc, media)
	sm := css.CascadeMedia(doc.Root, media, sheets)
	src := map[rune]bool{}
	var walk func(n *dom.Node, hidden bool)
	walk = func(n *dom.Node, hidden bool) {
		if n.Type == dom.Element {
			if st := sm[n]; st != nil && st.Display == css.DisplayNone {
				hidden = true
			}
			if n.Tag == "script" || n.Tag == "style" || n.Tag == "noscript" || n.Tag == "template" {
				return
			}
		}
		if n.Type == dom.Text && !hidden {
			for _, r := range n.Text {
				if !unicode.IsSpace(r) && r > ' ' && r != '\u00a0' && r != '\u200a' && r != '\u200b' {
					src[r] = true // a character, not a space of any width
				}
			}
		}
		for _, c := range n.Children {
			walk(c, hidden)
		}
	}
	walk(doc.Root, false)
	out, err := exec.Command("pdftotext", "-enc", "UTF-8", pdf, "-").Output()
	if err != nil {
		return -1, ""
	}
	got := map[rune]bool{}
	for _, r := range string(out) {
		got[r] = true
	}
	var lost []rune
	for r := range src {
		if !got[r] {
			lost = append(lost, r)
		}
	}
	sort.Slice(lost, func(i, j int) bool { return lost[i] < lost[j] })
	return len(lost), string(lost)
}
