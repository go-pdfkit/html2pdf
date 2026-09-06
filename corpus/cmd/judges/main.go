// Command judges runs every reference PDF consumer available on this machine
// over the corpus PDFs and reports whether each one opens, paginates, renders
// and extracts text the same way — "conformance per channel": a file that
// is the smallest is worth nothing if one reader in the field disagrees with
// the others about it.
//
// Judges, each skipped with a note when its binary is absent:
//
//	qpdf     structural check (qpdf --check)          — exit 0 clean, 3 warnings, 2 errors
//	poppler  pdfinfo / pdftoppm / pdftotext             — the reference the per-judge Δ is taken against
//	mupdf    mutool draw (png + txt), mutool info
//	gs       Ghostscript png16m + txtwrite
//	pdfium   pdfium_test --png --txt (Chrome's engine; PDFIUM_TEST=/path or -pdfium)
//	pdfjs    pdf.js under node (judges/pdfjs-*.mjs)     — Firefox's engine
//	quartz   sips (macOS ImageIO/Quartz, Preview's engine) — page 1 render only
//
// For each PDF and judge it records: pages reported, extracted-text length
// as a ratio of poppler's, and — on a sample of pages: the first, the middle
// and the last — how far its render is from poppler's at 96 dpi (share of
// pixels differing noticeably after both are box-downsampled to the same
// width), worst page reported. Because poppler is itself just one reader, a
// per-page consensus is computed too: the mean pairwise distance between all
// judges' renders of that page, so no single reader is privileged. Chrome's
// own PDFs of the same pages are judged alongside as the control: a judge
// that disagrees on those too is judge noise, not an html2pdf defect.
//
//	go run ./cmd/judges
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdfkit/html2pdf/corpus/internal/mdreport"
)

const (
	renderDPI   = 96
	pdfiumScale = "1.3333" // 96/72, so pdfium's render matches the others' size
	thumbWidth  = 400      // renders are compared after downsampling to this width
	diffThresh  = 48       // a pixel "differs" when its grey level moves by more than this, of 255
)

// pageDiff is one judge's render of one page measured against poppler's.
type pageDiff struct {
	MeanDiff float64 `json:"mean_diff"` // mean grey Δ, 0..255
	DiffPct  float64 `json:"diff_pct"`  // share of pixels with |Δ| > diffThresh, in %
}

// verdict is one judge's reading of one PDF.
type verdict struct {
	Judge     string           `json:"judge"`
	Skipped   bool             `json:"skipped,omitempty"`
	OK        bool             `json:"ok"`
	Err       string           `json:"err,omitempty"`
	Warnings  int              `json:"warnings"`
	Pages     int              `json:"pages"`      // 0 when the judge reports none
	TextChar  int              `json:"text_chars"` // -1 when the judge extracts none
	Renders   map[int]string   `json:"renders,omitempty"`
	Diffs     map[int]pageDiff `json:"diffs,omitempty"` // vs poppler, per sampled page
	WorstPct  float64          `json:"worst_pct"`
	WorstPage int              `json:"worst_page"`
	Ms        int64            `json:"ms"`
}

type fileResult struct {
	File         string          `json:"file"`
	Bytes        int64           `json:"bytes"`
	SampledPages []int           `json:"sampled_pages"`
	Verdicts     []verdict       `json:"verdicts"`
	Consensus    map[int]float64 `json:"consensus"` // per page: mean pairwise diff% among all judges' renders
	ConsensusMax float64         `json:"consensus_max"`
	ConsensusPg  int             `json:"consensus_page"`
}

type judge struct {
	name  string
	avail func() bool
	run   func(ctx context.Context, pdf, outDir string, pages []int) verdict
}

var (
	pdfiumBin string
	nodeDir   string // directory holding judges/pdfjs-*.mjs and node_modules
)

func have(bin string) bool { _, err := exec.LookPath(bin); return err == nil }

// runCmd runs a command, returning its stdout, stderr and error.
func runCmd(ctx context.Context, dir string, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err = cmd.Run()
	return o.String(), e.String(), err
}

func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// textLen counts the non-whitespace runes of an extraction. Whitespace is
// left out because judges disagree wildly on it for reasons that are not
// about the file — Ghostscript's txtwrite pads lines to reproduce column
// layout, pdfium separates every glyph run — while the glyphs they recover
// are what the comparison is about.
func textLen(s string) int {
	n := 0
	for _, r := range s {
		if r > ' ' {
			n++
		}
	}
	return n
}

var rePages = regexp.MustCompile(`(?mi)^Pages:\s+(\d+)`)

func pagesFrom(s string) int {
	if m := rePages.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// samplePages picks the pages whose renders are compared: the first, the
// middle and the last. Text and page counts cover every page regardless;
// this bounds the render work while still looking past page 1.
func samplePages(n int) []int {
	if n <= 1 {
		return []int{1}
	}
	set := map[int]bool{1: true, (n + 1) / 2: true, n: true}
	var out []int
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// ---- judges -------------------------------------------------------------

func judgeQpdf(ctx context.Context, pdf, outDir string, _ []int) verdict {
	v := verdict{Judge: "qpdf", TextChar: -1}
	out, errs, err := runCmd(ctx, "", "qpdf", "--check", pdf)
	all := out + errs
	v.Warnings = strings.Count(all, "WARNING")
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {
			v.OK = true // qpdf's "warnings only"
		} else {
			v.Err = firstLine(all, err)
		}
	} else {
		v.OK = true
	}
	return v
}

func judgePoppler(ctx context.Context, pdf, outDir string, pages []int) verdict {
	v := verdict{Judge: "poppler", Renders: map[int]string{}}
	info, e1, err := runCmd(ctx, "", "pdfinfo", pdf)
	if err != nil {
		v.Err = firstLine(e1, err)
		return v
	}
	v.Pages = pagesFrom(info)
	txt, e2, err := runCmd(ctx, "", "pdftotext", pdf, "-")
	if err != nil {
		v.Err = firstLine(e2, err)
		return v
	}
	v.TextChar = textLen(txt)
	v.Warnings = countLines(e1) + countLines(e2)
	for _, p := range pages {
		base := filepath.Join(outDir, fmt.Sprintf("poppler_p%d", p))
		ps := strconv.Itoa(p)
		_, e3, err := runCmd(ctx, "", "pdftoppm", "-png", "-r", strconv.Itoa(renderDPI), "-f", ps, "-l", ps, "-singlefile", pdf, base)
		if err != nil {
			v.Err = firstLine(e3, err)
			return v
		}
		v.Renders[p] = base + ".png"
		v.Warnings += countLines(e3)
	}
	v.OK = true
	return v
}

func judgeMupdf(ctx context.Context, pdf, outDir string, pages []int) verdict {
	v := verdict{Judge: "mupdf", Renders: map[int]string{}}
	info, e0, _ := runCmd(ctx, "", "mutool", "info", pdf)
	v.Pages = pagesFrom(info)
	txt, e1, err := runCmd(ctx, "", "mutool", "draw", "-q", "-F", "txt", "-o", "-", pdf)
	if err != nil {
		v.Err = firstLine(e1, err)
		return v
	}
	v.TextChar = textLen(txt)
	v.Warnings = countLines(e0) + countLines(e1)
	for _, p := range pages {
		out := filepath.Join(outDir, fmt.Sprintf("mupdf_p%d.png", p))
		_, e2, err := runCmd(ctx, "", "mutool", "draw", "-q", "-r", strconv.Itoa(renderDPI), "-o", out, pdf, strconv.Itoa(p))
		if err != nil {
			v.Err = firstLine(e2, err)
			return v
		}
		v.Renders[p] = out
		v.Warnings += countLines(e2)
	}
	v.OK = true
	return v
}

func judgeGs(ctx context.Context, pdf, outDir string, pages []int) verdict {
	v := verdict{Judge: "gs", Renders: map[int]string{}}
	txtFile := filepath.Join(outDir, "gs.txt")
	o1, e1, err := runCmd(ctx, "", "gs", "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-sDEVICE=txtwrite", "-sOutputFile="+txtFile, pdf)
	if err != nil {
		v.Err = firstLine(o1+e1, err)
		return v
	}
	if b, err := os.ReadFile(txtFile); err == nil {
		v.TextChar = textLen(string(b))
	}
	v.Warnings = countLines(o1 + e1)
	for _, p := range pages {
		out := filepath.Join(outDir, fmt.Sprintf("gs_p%d.png", p))
		ps := strconv.Itoa(p)
		o2, e2, err := runCmd(ctx, "", "gs", "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-sDEVICE=png16m", "-r"+strconv.Itoa(renderDPI),
			"-dFirstPage="+ps, "-dLastPage="+ps, "-sOutputFile="+out, pdf)
		if err != nil {
			v.Err = firstLine(o2+e2, err)
			return v
		}
		v.Renders[p] = out
		v.Warnings += countLines(o2 + e2)
	}
	v.OK = true
	return v
}

var pdfiumNoise = regexp.MustCompile(`(?m)^(Processing PDF file .*|Processed \d+ pages\.)\n?`)

func judgePdfium(ctx context.Context, pdf, outDir string, pages []int) verdict {
	v := verdict{Judge: "pdfium", Renders: map[int]string{}}
	// pdfium_test writes <file>.<page>.png / .txt beside the input; work on a
	// copy in outDir so the corpus tree stays clean.
	work := filepath.Join(outDir, "pdfium.pdf")
	b, err := os.ReadFile(pdf)
	if err != nil {
		v.Err = err.Error()
		return v
	}
	if err := os.WriteFile(work, b, 0o644); err != nil {
		v.Err = err.Error()
		return v
	}
	defer os.Remove(work)
	o1, e1, err := runCmd(ctx, outDir, pdfiumBin, "--txt", "pdfium.pdf")
	if err != nil {
		v.Err = firstLine(o1+e1, err)
		return v
	}
	txts, _ := filepath.Glob(filepath.Join(outDir, "pdfium.pdf.*.txt"))
	v.Pages = len(txts)
	total := 0
	for _, t := range txts {
		if tb, err := os.ReadFile(t); err == nil {
			total += textLen(utf32leToString(tb))
		}
		os.Remove(t)
	}
	v.TextChar = total
	// pdfium_test narrates on stderr ("Processing PDF file x.", "Processed N
	// pages.") — progress, not warnings; only anything else counts.
	v.Warnings = countLines(pdfiumNoise.ReplaceAllString(e1, ""))
	for _, p := range pages {
		o2, e2, err := runCmd(ctx, outDir, pdfiumBin, "--png", "--scale="+pdfiumScale, "--pages="+strconv.Itoa(p-1), "pdfium.pdf")
		if err != nil {
			v.Err = firstLine(o2+e2, err)
			return v
		}
		src := filepath.Join(outDir, fmt.Sprintf("pdfium.pdf.%d.png", p-1))
		out := filepath.Join(outDir, fmt.Sprintf("pdfium_p%d.png", p))
		if err := os.Rename(src, out); err != nil {
			v.Err = "no render for page " + strconv.Itoa(p)
			return v
		}
		v.Renders[p] = out
		v.Warnings += countLines(pdfiumNoise.ReplaceAllString(e2, ""))
	}
	v.OK = true
	return v
}

// utf32leToString decodes pdfium_test's --txt output — UTF-32LE with a
// byte-order mark (FF FE 00 00; verified with xxd, four bytes per character)
// — so its length is counted in characters like every other judge's, not in
// bytes, which would read as four times the text.
func utf32leToString(b []byte) string {
	if len(b) >= 4 && b[0] == 0xFF && b[1] == 0xFE && b[2] == 0 && b[3] == 0 {
		b = b[4:]
	}
	r := make([]rune, 0, len(b)/4)
	for i := 0; i+3 < len(b); i += 4 {
		r = append(r, rune(uint32(b[i])|uint32(b[i+1])<<8|uint32(b[i+2])<<16|uint32(b[i+3])<<24))
	}
	return string(r)
}

var rePdfjsPages = regexp.MustCompile(`(?m)^pages (\d+)`)

func judgePdfjs(ctx context.Context, pdf, outDir string, pages []int) verdict {
	v := verdict{Judge: "pdfjs", Renders: map[int]string{}}
	out, e1, err := runCmd(ctx, nodeDir, "node", "pdfjs-text.mjs", pdf)
	if err != nil {
		v.Err = firstLine(e1, err)
		return v
	}
	if m := rePdfjsPages.FindStringSubmatch(out); m != nil {
		v.Pages, _ = strconv.Atoi(m[1])
		out = out[len(m[0]):]
	}
	v.TextChar = textLen(out)
	v.Warnings = countLines(e1)
	for _, p := range pages {
		render := filepath.Join(outDir, fmt.Sprintf("pdfjs_p%d.png", p))
		_, e2, err := runCmd(ctx, nodeDir, "node", "pdfjs-render.mjs", pdf, render, strconv.Itoa(p), pdfiumScale)
		if err != nil {
			v.Err = firstLine(e2, err)
			return v
		}
		v.Renders[p] = render
		v.Warnings += countLines(e2)
	}
	v.OK = true
	return v
}

// judgeQuartz renders page 1 only: sips has no page selection.
func judgeQuartz(ctx context.Context, pdf, outDir string, _ []int) verdict {
	v := verdict{Judge: "quartz", TextChar: -1, Renders: map[int]string{}}
	out := filepath.Join(outDir, "quartz_p1.png")
	o, e, err := runCmd(ctx, "", "sips", "-s", "format", "png", pdf, "--out", out)
	if err != nil {
		v.Err = firstLine(o+e, err)
		return v
	}
	v.Renders[1] = out
	v.Warnings = strings.Count(o+e, "Error") + strings.Count(o+e, "Warning")
	v.OK = true
	return v
}

func firstLine(s string, err error) string {
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if len(l) > 120 {
				l = l[:120] + "…"
			}
			return l
		}
	}
	return err.Error()
}

// ---- render comparison ----------------------------------------------------

// greyThumb decodes a PNG, composites it over white, box-downsamples it to
// width w and returns it as 8-bit grey rows.
func greyThumb(path string, w int) ([][]uint8, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil, fmt.Errorf("empty image")
	}
	h := b.Dy() * w / b.Dx()
	if h == 0 {
		h = 1
	}
	rows := make([][]uint8, h)
	for y := 0; y < h; y++ {
		rows[y] = make([]uint8, w)
		y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+(y+1)*b.Dy()/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+(x+1)*b.Dx()/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum, n int
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					// Composite over white first: Quartz (sips) renders a page
					// on a transparent background, and a transparent pixel
					// converted straight to grey is black — every such render
					// would read as a 99% mismatch against an opaque one.
					r, g, bl, a := img.At(xx, yy).RGBA()
					if a < 0xffff {
						r += 0xffff - a
						g += 0xffff - a
						bl += 0xffff - a
					}
					grey := (19595*r + 38470*g + 7471*bl + 1<<15) >> 24 // 0..255
					sum += int(grey)
					n++
				}
			}
			rows[y][x] = uint8(sum / n)
		}
	}
	return rows, nil
}

// thumbCache keeps each render's downsampled grey rows so the pairwise
// consensus doesn't decode the same PNG once per pair.
var thumbCache = map[string][][]uint8{}

func thumb(path string) ([][]uint8, error) {
	if t, ok := thumbCache[path]; ok {
		return t, nil
	}
	t, err := greyThumb(path, thumbWidth)
	if err == nil {
		thumbCache[path] = t
	}
	return t, err
}

// compareRenders returns the mean absolute grey difference and the share of
// pixels differing by more than diffThresh between two renders of the same
// page, over the height both cover.
func compareRenders(a, b string) (mean, pct float64, err error) {
	ra, err := thumb(a)
	if err != nil {
		return 0, 0, err
	}
	rb, err := thumb(b)
	if err != nil {
		return 0, 0, err
	}
	h := len(ra)
	if len(rb) < h {
		h = len(rb)
	}
	var sum, big, n int
	for y := 0; y < h; y++ {
		for x := 0; x < thumbWidth; x++ {
			d := int(ra[y][x]) - int(rb[y][x])
			if d < 0 {
				d = -d
			}
			sum += d
			if d > diffThresh {
				big++
			}
			n++
		}
	}
	if n == 0 {
		return 0, 0, fmt.Errorf("no overlap")
	}
	return float64(sum) / float64(n), 100 * float64(big) / float64(n), nil
}

// score fills each verdict's per-page distance to poppler and its worst
// page, then the per-page consensus: the mean pairwise distance between every
// two judges' renders of that page, poppler included as just one of them.
func score(fr *fileResult) {
	var ref *verdict
	for i := range fr.Verdicts {
		if fr.Verdicts[i].Judge == "poppler" {
			ref = &fr.Verdicts[i]
		}
	}
	for i := range fr.Verdicts {
		v := &fr.Verdicts[i]
		if ref == nil || v == ref || len(v.Renders) == 0 {
			continue
		}
		v.Diffs = map[int]pageDiff{}
		for p, path := range v.Renders {
			rp, ok := ref.Renders[p]
			if !ok {
				continue
			}
			m, pct, err := compareRenders(rp, path)
			if err != nil {
				v.Err = "compare: " + err.Error()
				continue
			}
			v.Diffs[p] = pageDiff{m, pct}
			if pct >= v.WorstPct {
				v.WorstPct, v.WorstPage = pct, p
			}
		}
	}
	fr.Consensus = map[int]float64{}
	for _, p := range fr.SampledPages {
		var paths []string
		for _, v := range fr.Verdicts {
			if path, ok := v.Renders[p]; ok && v.OK {
				paths = append(paths, path)
			}
		}
		var sum float64
		var n int
		for i := 0; i < len(paths); i++ {
			for j := i + 1; j < len(paths); j++ {
				if _, pct, err := compareRenders(paths[i], paths[j]); err == nil {
					sum += pct
					n++
				}
			}
		}
		if n > 0 {
			fr.Consensus[p] = sum / float64(n)
			if fr.Consensus[p] >= fr.ConsensusMax {
				fr.ConsensusMax, fr.ConsensusPg = fr.Consensus[p], p
			}
		}
	}
}

// ---- driver -------------------------------------------------------------

func version(bin string, args ...string) string {
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return "?"
	}
	l := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if len(l) > 60 {
		l = l[:60]
	}
	return l
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func run() int {
	glob := flag.String("pdfs", "out/*.pdf,out/bench/*.pdf", "comma-separated globs of PDFs to judge")
	outDir := flag.String("out", "out/judges", "directory for per-judge renders")
	resultsPath := flag.String("results", "judges.json", "JSON results path")
	reportPath := flag.String("report", "JUDGES.md", "Markdown report path")
	flag.StringVar(&pdfiumBin, "pdfium", envOr("PDFIUM_TEST", ""), "pdfium_test binary (Chrome's engine); PDFIUM_TEST env")
	flag.StringVar(&nodeDir, "nodedir", "judges", "directory with pdfjs-*.mjs and node_modules")
	timeout := flag.Duration("timeout", 180*time.Second, "per judge per file")
	flag.Parse()

	judges := []judge{
		{"qpdf", func() bool { return have("qpdf") }, judgeQpdf},
		{"poppler", func() bool { return have("pdfinfo") && have("pdftoppm") && have("pdftotext") }, judgePoppler},
		{"mupdf", func() bool { return have("mutool") }, judgeMupdf},
		{"gs", func() bool { return have("gs") }, judgeGs},
		{"pdfium", func() bool { _, err := os.Stat(pdfiumBin); return pdfiumBin != "" && err == nil }, judgePdfium},
		{"pdfjs", func() bool {
			_, err := os.Stat(filepath.Join(nodeDir, "node_modules", "pdfjs-dist"))
			return have("node") && err == nil
		}, judgePdfjs},
		{"quartz", func() bool { return have("sips") }, judgeQuartz},
	}

	var files []string
	for _, g := range strings.Split(*glob, ",") {
		m, _ := filepath.Glob(strings.TrimSpace(g))
		files = append(files, m...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "judges: no PDFs matched")
		return 1
	}
	nodeDir, _ = filepath.Abs(nodeDir)

	var results []fileResult
	for _, f := range files {
		abs, _ := filepath.Abs(f)
		st, _ := os.Stat(abs)
		fr := fileResult{File: f, Bytes: st.Size()}
		// Absolute: pdfium and pdf.js run with another working directory.
		dir, _ := filepath.Abs(filepath.Join(*outDir, strings.TrimSuffix(filepath.Base(f), ".pdf")))
		os.MkdirAll(dir, 0o755)
		fmt.Fprintf(os.Stderr, "%s\n", f)
		// Page count for the sample comes from poppler, which runs before the
		// renderers; until then only page 1 is known to exist.
		pages := []int{1}
		for _, j := range judges {
			if !j.avail() {
				fr.Verdicts = append(fr.Verdicts, verdict{Judge: j.name, Skipped: true, TextChar: -1})
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			t0 := time.Now()
			v := j.run(ctx, abs, dir, pages)
			v.Ms = time.Since(t0).Milliseconds()
			cancel()
			if v.Judge == "poppler" && v.Pages > 0 && len(pages) == 1 && v.Pages > 1 {
				// Re-render poppler on the full sample now that the count is known.
				pages = samplePages(v.Pages)
				fr.SampledPages = pages
				ctx, cancel := context.WithTimeout(context.Background(), *timeout)
				v = j.run(ctx, abs, dir, pages)
				cancel()
			}
			if fr.SampledPages == nil {
				fr.SampledPages = pages
			}
			fmt.Fprintf(os.Stderr, "  %-8s ok=%v pages=%d text=%d warn=%d %dms %s\n",
				v.Judge, v.OK, v.Pages, v.TextChar, v.Warnings, v.Ms, v.Err)
			fr.Verdicts = append(fr.Verdicts, v)
		}
		score(&fr)
		for _, v := range fr.Verdicts {
			if len(v.Diffs) > 0 {
				fmt.Fprintf(os.Stderr, "  %-8s worst Δ%.1f%% on p%d\n", v.Judge, v.WorstPct, v.WorstPage)
			}
		}
		fmt.Fprintf(os.Stderr, "  consensus max %.1f%% on p%d (pages %v)\n", fr.ConsensusMax, fr.ConsensusPg, fr.SampledPages)
		results = append(results, fr)
	}

	if f, err := os.Create(*resultsPath); err == nil {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		f.Close()
	}
	return writeReport(*reportPath, results, judges)
}

func writeReport(path string, results []fileResult, judges []judge) int {
	var b strings.Builder
	fmt.Fprintf(&b, "# html2pdf output judged by every reader on this machine — %s\n\n", time.Now().UTC().Format("2006-01-02"))
	b.WriteString("Judges: ")
	var names []string
	for _, j := range judges {
		if j.avail() {
			names = append(names, j.name)
		}
	}
	b.WriteString(strings.Join(names, ", "))
	fmt.Fprintf(&b, ". Versions: poppler %s; mupdf %s; gs %s; qpdf %s; pdf.js %s; macOS %s.\n\n",
		version("pdftoppm", "-v"), version("mutool", "-v"), version("gs", "--version"), version("qpdf", "--version"),
		pdfjsVersion(), version("sw_vers", "-productVersion"))
	fmt.Fprintf(&b, "Cell format: `pages · text ratio · Δworst (page)` — pages the judge reports (– if none), "+
		"its extracted-text length as a ratio of poppler's (– if it extracts none), and the largest distance of its "+
		"renders from poppler's over the sampled pages (first, middle, last, at %d dpi: share of pixels whose grey "+
		"level moves by more than %d/255 after both are downsampled to %d px wide), with the page it happened on. "+
		"`consensus` is the mean pairwise distance between all judges' renders of a page, worst page — no reader "+
		"privileged. ⚠n = n warning lines on stderr; ❌ = the judge failed to process the file.\n\n",
		renderDPI, diffThresh, thumbWidth)

	b.WriteString("| PDF | Bytes |")
	for _, n := range names {
		b.WriteString(" " + n + " |")
	}
	b.WriteString(" consensus |\n|---|---|")
	for range names {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	for _, fr := range results {
		fmt.Fprintf(&b, "| %s | %s |", fr.File, fmtBytes(fr.Bytes))
		var popplerText int
		for _, v := range fr.Verdicts {
			if v.Judge == "poppler" {
				popplerText = v.TextChar
			}
		}
		for _, v := range fr.Verdicts {
			if v.Skipped {
				continue
			}
			b.WriteString(" " + cell(v, popplerText) + " |")
		}
		fmt.Fprintf(&b, " %.1f%% (p%d) |\n", fr.ConsensusMax, fr.ConsensusPg)
	}
	b.WriteString("\n")
	if err := mdreport.Write(path, b.String()); err != nil {
		fmt.Fprintf(os.Stderr, "judges: report: %v\n", err)
		return 1
	}
	return 0
}

func cell(v verdict, popplerText int) string {
	if !v.OK {
		return "❌ " + v.Err
	}
	var parts []string
	if v.Pages > 0 {
		parts = append(parts, strconv.Itoa(v.Pages)+"p")
	} else {
		parts = append(parts, "–")
	}
	switch {
	case v.TextChar < 0:
		parts = append(parts, "–")
	case popplerText > 0:
		parts = append(parts, fmt.Sprintf("%.3f", float64(v.TextChar)/float64(popplerText)))
	default:
		parts = append(parts, strconv.Itoa(v.TextChar))
	}
	switch {
	case v.Judge == "poppler":
		parts = append(parts, "ref")
	case len(v.Diffs) > 0 && v.Judge == "quartz":
		parts = append(parts, fmt.Sprintf("Δ%.1f%% (p1 only)", v.WorstPct))
	case len(v.Diffs) > 0:
		parts = append(parts, fmt.Sprintf("Δ%.1f%% (p%d)", v.WorstPct, v.WorstPage))
	default:
		parts = append(parts, "–")
	}
	s := "✅ " + strings.Join(parts, " · ")
	if v.Warnings > 0 {
		s += fmt.Sprintf(" ⚠%d", v.Warnings)
	}
	return s
}

func fmtBytes(n int64) string {
	switch {
	case n >= 1e6:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.0f KB", float64(n)/1e3)
	}
	return fmt.Sprintf("%d B", n)
}

func pdfjsVersion() string {
	b, err := os.ReadFile(filepath.Join(nodeDir, "node_modules", "pdfjs-dist", "package.json"))
	if err != nil {
		return "?"
	}
	var p struct{ Version string }
	if json.Unmarshal(b, &p) != nil {
		return "?"
	}
	return p.Version
}

func main() { os.Exit(run()) }
