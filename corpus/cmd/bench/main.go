// Command bench times html2pdf against headless Chrome's own print-to-PDF on
// the same inputs — the corpus URLs plus self-contained local fixtures — and
// writes BENCH.md, in the spirit of engine-webengine/bench.
//
// Both tools run as child processes under /usr/bin/time -l, so the numbers
// are symmetric: wall-clock from "give me a PDF of this input" to the file on
// disk, process start-up and (for a URL) fetch included, plus peak resident
// memory. Runs are interleaved A/B/A/B… and reported as medians, so a
// background load hits both sides alike; the load average is recorded in the
// report header because it cannot be removed.
//
//	go run ./cmd/bench -html2pdf /tmp/html2pdf-bin -n 5
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdfkit/html2pdf/corpus/internal/mdreport"
	"github.com/go-pdfkit/html2pdf/corpus/internal/pdfstat"
)

const defaultChrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// input is one thing to render: a URL, or a local file (path set, url empty).
type input struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
	File string `json:"file,omitempty"`
}

// sample is one timed run of one tool.
type sample struct {
	RealSec float64 `json:"real_sec"`
	MaxRSS  int64   `json:"max_rss_bytes"`
	Err     string  `json:"err,omitempty"`
}

// toolResult aggregates a tool's runs on one input.
type toolResult struct {
	Samples   []sample `json:"samples"`
	MedianSec float64  `json:"median_sec"`
	MedianRSS int64    `json:"median_rss_bytes"`
	Failures  int      `json:"failures"`
	PDFBytes  int64    `json:"pdf_bytes"`
	Pages     int      `json:"pages"`
	TextChars int      `json:"text_chars"`
	Links     int      `json:"links"` // link annotations in the PDF
}

type result struct {
	Input    input      `json:"input"`
	HTML2PDF toolResult `json:"html2pdf"`
	Chrome   toolResult `json:"chrome"`
}

var (
	reReal = regexp.MustCompile(`(?m)^\s*([\d.]+)\s+real`)
	reRSS  = regexp.MustCompile(`(?m)^\s*(\d+)\s+maximum resident set size`)
)

// timed runs bin with args under /usr/bin/time -l and parses wall-clock
// seconds and peak RSS (macOS reports it in bytes) from its stderr summary.
func timed(bin string, args ...string) sample {
	cmd := exec.Command("/usr/bin/time", append([]string{"-l", bin}, args...)...)
	out, err := cmd.CombinedOutput()
	s := sample{}
	if m := reReal.FindSubmatch(out); m != nil {
		s.RealSec, _ = strconv.ParseFloat(string(m[1]), 64)
	}
	if m := reRSS.FindSubmatch(out); m != nil {
		s.MaxRSS, _ = strconv.ParseInt(string(m[1]), 10, 64)
	}
	if err != nil {
		s.Err = err.Error()
	}
	return s
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func medianInt(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]int64(nil), xs...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	return c[len(c)/2]
}

func finish(t *toolResult, pdf string) {
	var secs []float64
	var rss []int64
	for _, s := range t.Samples {
		if s.Err != "" {
			t.Failures++
			continue
		}
		secs = append(secs, s.RealSec)
		rss = append(rss, s.MaxRSS)
	}
	t.MedianSec = median(secs)
	t.MedianRSS = medianInt(rss)
	if st, err := os.Stat(pdf); err == nil {
		t.PDFBytes = st.Size()
	}
	if b, err := os.ReadFile(pdf); err == nil {
		t.Links = pdfstat.CountLinks(b)
	}
	t.Pages = pdfPages(pdf)
	t.TextChars = pdfTextChars(pdf)
}

func pdfPages(path string) int {
	out, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
			return n
		}
	}
	return 0
}

func pdfTextChars(path string) int {
	out, err := exec.Command("pdftotext", path, "-").Output()
	if err != nil {
		return -1
	}
	return len(strings.TrimSpace(string(out)))
}

func readURLs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

func slug(s string) string {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.Trim(s, "/")
	return strings.NewReplacer("/", "-", ":", "-", "(", "", ")", "", ".", "-", " ", "-").Replace(s)
}

func sysctl(key string) string {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(out))
}

func chromeVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(out))
}

func fmtMB(b int64) string { return fmt.Sprintf("%.1f MB", float64(b)/1e6) }

func run() int {
	urlsPath := flag.String("urls", "urls.txt", "corpus URL list (blank to skip URLs)")
	files := flag.String("files", "fixtures/longdoc.html,fixtures/breaks.html", "comma-separated local HTML files (self-contained, no network for either tool)")
	n := flag.Int("n", 5, "runs per tool per input, interleaved; medians are reported")
	chrome := flag.String("chrome", envOr("CHROME_BIN", defaultChrome), "Chrome/Chromium binary")
	bin := flag.String("html2pdf", "html2pdf", "html2pdf CLI binary (build it first so timings exclude compilation)")
	budget := flag.Int("budget", 8000, "Chrome --virtual-time-budget in ms (lets a page's resources settle)")
	outDir := flag.String("out", "out/bench", "directory for the rendered PDFs")
	resultsPath := flag.String("results", "bench.json", "JSON results path")
	reportPath := flag.String("report", "BENCH.md", "Markdown report path")
	flag.Parse()

	var inputs []input
	if *urlsPath != "" {
		urls, err := readURLs(*urlsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bench: %v\n", err)
			return 1
		}
		for _, u := range urls {
			inputs = append(inputs, input{Name: u, URL: u})
		}
	}
	for _, f := range strings.Split(*files, ",") {
		if f = strings.TrimSpace(f); f != "" {
			abs, err := filepath.Abs(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "bench: %v\n", err)
				return 1
			}
			inputs = append(inputs, input{Name: f, File: abs})
		}
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "bench: %v\n", err)
		return 1
	}

	var results []result
	for _, in := range inputs {
		r := result{Input: in}
		aPDF := filepath.Join(*outDir, slug(in.Name)+".html2pdf.pdf")
		bPDF := filepath.Join(*outDir, slug(in.Name)+".chrome.pdf")
		var aArgs, bArgs []string
		if in.URL != "" {
			aArgs = []string{"-url", in.URL, "-out", aPDF}
			bArgs = []string{in.URL}
		} else {
			aArgs = []string{"-in", in.File, "-out", aPDF}
			bArgs = []string{"file://" + in.File}
		}
		bArgs = append([]string{"--headless", "--disable-gpu", "--no-sandbox", "--no-pdf-header-footer",
			fmt.Sprintf("--virtual-time-budget=%d", *budget), "--print-to-pdf=" + bPDF}, bArgs...)

		fmt.Fprintf(os.Stderr, "%s\n", in.Name)
		for i := 0; i < *n; i++ {
			os.Remove(aPDF)
			os.Remove(bPDF)
			sa := timed(*bin, aArgs...)
			if sa.Err == "" {
				if st, err := os.Stat(aPDF); err != nil || st.Size() == 0 {
					sa.Err = "no output"
				}
			}
			sb := timed(*chrome, bArgs...)
			if sb.Err == "" {
				if st, err := os.Stat(bPDF); err != nil || st.Size() == 0 {
					sb.Err = "no output"
				}
			}
			fmt.Fprintf(os.Stderr, "  run %d: html2pdf %.2fs  chrome %.2fs\n", i+1, sa.RealSec, sb.RealSec)
			r.HTML2PDF.Samples = append(r.HTML2PDF.Samples, sa)
			r.Chrome.Samples = append(r.Chrome.Samples, sb)
		}
		finish(&r.HTML2PDF, aPDF)
		finish(&r.Chrome, bPDF)
		results = append(results, r)
	}

	if f, err := os.Create(*resultsPath); err == nil {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		f.Close()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# html2pdf vs headless Chrome — %s\n\n", time.Now().UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "Machine: %s, %s cores; load average at start: %s. Chrome: %s. "+
		"%d runs per tool per input, interleaved; medians. Both tools timed as child processes under "+
		"`/usr/bin/time -l` (wall-clock to PDF on disk, start-up and any fetch included; RSS = peak resident set).\n\n",
		sysctl("machdep.cpu.brand_string"), sysctl("hw.ncpu"), sysctl("vm.loadavg"), chromeVersion(*chrome), *n)
	b.WriteString("| Input | html2pdf | Chrome | Chrome ÷ html2pdf | PDF html2pdf | PDF Chrome | Pages | Text chars | Links | RSS html2pdf | RSS Chrome |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range results {
		a, c := r.HTML2PDF, r.Chrome
		ratio := "—"
		if a.MedianSec > 0 && c.MedianSec > 0 {
			ratio = fmt.Sprintf("%.1f×", c.MedianSec/a.MedianSec)
		}
		as, cs := fmt.Sprintf("%.2f s", a.MedianSec), fmt.Sprintf("%.2f s", c.MedianSec)
		if a.Failures > 0 {
			as += fmt.Sprintf(" (%d fail)", a.Failures)
		}
		if c.Failures > 0 {
			cs += fmt.Sprintf(" (%d fail)", c.Failures)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %d / %d | %d / %d | %d / %d | %s | %s |\n",
			r.Input.Name, as, cs, ratio, fmtMB(a.PDFBytes), fmtMB(c.PDFBytes),
			a.Pages, c.Pages, a.TextChars, c.TextChars, a.Links, c.Links, fmtMB(a.MedianRSS), fmtMB(c.MedianRSS))
	}
	b.WriteString("\n")
	if err := mdreport.Write(*reportPath, b.String()); err != nil {
		fmt.Fprintf(os.Stderr, "bench: report: %v\n", err)
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() { os.Exit(run()) }
