// Command judges runs every reference PDF consumer available on this machine
// over the corpus PDFs and reports whether each one opens, paginates, renders
// and extracts text the same way — "conformance per channel": a file that
// is the smallest is worth nothing if one reader in the field disagrees with
// the others about it.
//
// Judges, each skipped with a note when its binary is absent:
//
//	qpdf     structural check (qpdf --check)          — exit 0 clean, 3 warnings, 2 errors
//	poppler  pdfinfo / pdftoppm / pdftotext             — the reference the others are compared to
//	mupdf    mutool draw (png + txt), mutool info
//	gs       Ghostscript png16m + txtwrite
//	pdfium   pdfium_test --png --txt (Chrome's engine; PDFIUM_TEST=/path or -pdfium)
//	pdfjs    pdf.js under node (judges/pdfjs-*.mjs)     — Firefox's engine
//	quartz   sips (macOS ImageIO/Quartz, Preview's engine) — page 1 render only
//
// For each PDF and judge it records: pages reported, extracted-text length
// as a ratio of poppler's, and how far its page-1 render is from poppler's
// (mean absolute grey difference and the share of pixels differing
// noticeably, after both are box-downsampled to the same width). Chrome's
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

// verdict is one judge's reading of one PDF.
type verdict struct {
	Judge    string  `json:"judge"`
	Skipped  bool    `json:"skipped,omitempty"`
	OK       bool    `json:"ok"`
	Err      string  `json:"err,omitempty"`
	Warnings int     `json:"warnings"`
	Pages    int     `json:"pages"`      // 0 when the judge reports none
	TextChar int     `json:"text_chars"` // -1 when the judge extracts none
	Render   string  `json:"render,omitempty"`
	MeanDiff float64 `json:"mean_diff"` // vs poppler's page-1 render, 0..255
	DiffPct  float64 `json:"diff_pct"`  // share of pixels with |Δ| > 48, in %
	Ms       int64   `json:"ms"`
}

type fileResult struct {
	File     string    `json:"file"`
	Bytes    int64     `json:"bytes"`
	Verdicts []verdict `json:"verdicts"`
}

type judge struct {
	name  string
	avail func() bool
	run   func(ctx context.Context, pdf, outDir string) verdict
}

var (
	pdfiumBin string
	nodeDir   string // directory holding judges/pdfjs-*.mjs and node_modules
)

func have(bin string) bool { _, err := exec.LookPath(bin); return err == nil }

// runCmd runs a command, returning combined stderr text and the error.
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

// ---- judges -------------------------------------------------------------

func judgeQpdf(ctx context.Context, pdf, outDir string) verdict {
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

func judgePoppler(ctx context.Context, pdf, outDir string) verdict {
	v := verdict{Judge: "poppler"}
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
	base := filepath.Join(outDir, "poppler")
	_, e3, err := runCmd(ctx, "", "pdftoppm", "-png", "-r", "72", "-f", "1", "-l", "1", "-singlefile", pdf, base)
	if err != nil {
		v.Err = firstLine(e3, err)
		return v
	}
	v.Render = base + ".png"
	v.Warnings = countLines(e1) + countLines(e2) + countLines(e3)
	v.OK = true
	return v
}

func judgeMupdf(ctx context.Context, pdf, outDir string) verdict {
	v := verdict{Judge: "mupdf"}
	info, e0, _ := runCmd(ctx, "", "mutool", "info", pdf)
	v.Pages = pagesFrom(info)
	txt, e1, err := runCmd(ctx, "", "mutool", "draw", "-q", "-F", "txt", "-o", "-", pdf)
	if err != nil {
		v.Err = firstLine(e1, err)
		return v
	}
	v.TextChar = textLen(txt)
	out := filepath.Join(outDir, "mupdf.png")
	_, e2, err := runCmd(ctx, "", "mutool", "draw", "-q", "-r", "72", "-o", out, pdf, "1")
	if err != nil {
		v.Err = firstLine(e2, err)
		return v
	}
	v.Render = out
	v.Warnings = countLines(e0) + countLines(e1) + countLines(e2)
	v.OK = true
	return v
}

func judgeGs(ctx context.Context, pdf, outDir string) verdict {
	v := verdict{Judge: "gs"}
	txtFile := filepath.Join(outDir, "gs.txt")
	o1, e1, err := runCmd(ctx, "", "gs", "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-sDEVICE=txtwrite", "-sOutputFile="+txtFile, pdf)
	if err != nil {
		v.Err = firstLine(o1+e1, err)
		return v
	}
	if b, err := os.ReadFile(txtFile); err == nil {
		v.TextChar = textLen(string(b))
	}
	out := filepath.Join(outDir, "gs.png")
	o2, e2, err := runCmd(ctx, "", "gs", "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-sDEVICE=png16m", "-r72",
		"-dFirstPage=1", "-dLastPage=1", "-sOutputFile="+out, pdf)
	if err != nil {
		v.Err = firstLine(o2+e2, err)
		return v
	}
	v.Render = out
	v.Warnings = countLines(o1+e1) + countLines(o2+e2)
	v.OK = true
	return v
}

func judgePdfium(ctx context.Context, pdf, outDir string) verdict {
	v := verdict{Judge: "pdfium"}
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
	o2, e2, err := runCmd(ctx, outDir, pdfiumBin, "--png", "--pages=0", "pdfium.pdf")
	if err != nil {
		v.Err = firstLine(o2+e2, err)
		return v
	}
	if pngs, _ := filepath.Glob(filepath.Join(outDir, "pdfium.pdf.0.png")); len(pngs) == 1 {
		out := filepath.Join(outDir, "pdfium.png")
		os.Rename(pngs[0], out)
		v.Render = out
	}
	os.Remove(work)
	// pdfium_test narrates on stderr ("Processing PDF file x.", "Processed N
	// pages.") — progress, not warnings; only anything else counts.
	v.Warnings = countLines(pdfiumNoise.ReplaceAllString(e1, "")) + countLines(pdfiumNoise.ReplaceAllString(e2, ""))
	v.OK = v.Render != ""
	if !v.OK {
		v.Err = "no page render produced"
	}
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

var (
	rePdfjsPages = regexp.MustCompile(`(?m)^pages (\d+)`)
	pdfiumNoise  = regexp.MustCompile(`(?m)^(Processing PDF file .*|Processed \d+ pages\.)\n?`)
)

func judgePdfjs(ctx context.Context, pdf, outDir string) verdict {
	v := verdict{Judge: "pdfjs"}
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
	render := filepath.Join(outDir, "pdfjs.png")
	_, e2, err := runCmd(ctx, nodeDir, "node", "pdfjs-render.mjs", pdf, render, "1", "1")
	if err != nil {
		v.Err = firstLine(e2, err)
		return v
	}
	v.Render = render
	v.Warnings = countLines(e1) + countLines(e2)
	v.OK = true
	return v
}

func judgeQuartz(ctx context.Context, pdf, outDir string) verdict {
	v := verdict{Judge: "quartz", TextChar: -1}
	out := filepath.Join(outDir, "quartz.png")
	o, e, err := runCmd(ctx, "", "sips", "-s", "format", "png", pdf, "--out", out)
	if err != nil {
		v.Err = firstLine(o+e, err)
		return v
	}
	v.Render = out
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

// greyThumb decodes a PNG, box-downsamples it to width w and returns it as
// 8-bit grey rows.
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
					r, g, b, a := img.At(xx, yy).RGBA()
					if a < 0xffff {
						r += 0xffff - a
						g += 0xffff - a
						b += 0xffff - a
					}
					y := (19595*r + 38470*g + 7471*b + 1<<15) >> 24 // 0..255
					sum += int(y)
					n++
				}
			}
			rows[y][x] = uint8(sum / n)
		}
	}
	return rows, nil
}

// compareRenders returns the mean absolute grey difference and the share of
// pixels differing by more than 48/255 between two renders of the same page,
// over the height both cover.
func compareRenders(a, b string) (mean, pct float64, err error) {
	const w = 300
	ra, err := greyThumb(a, w)
	if err != nil {
		return 0, 0, err
	}
	rb, err := greyThumb(b, w)
	if err != nil {
		return 0, 0, err
	}
	h := len(ra)
	if len(rb) < h {
		h = len(rb)
	}
	var sum, big, n int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := int(ra[y][x]) - int(rb[y][x])
			if d < 0 {
				d = -d
			}
			sum += d
			if d > 48 {
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
	timeout := flag.Duration("timeout", 120*time.Second, "per judge per file")
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
	absNode, _ := filepath.Abs(nodeDir)
	nodeDir = absNode

	var results []fileResult
	for _, f := range files {
		abs, _ := filepath.Abs(f)
		st, _ := os.Stat(abs)
		fr := fileResult{File: f, Bytes: st.Size()}
		// Absolute: pdfium and pdf.js run with another working directory.
		dir, _ := filepath.Abs(filepath.Join(*outDir, strings.TrimSuffix(filepath.Base(f), ".pdf")))
		os.MkdirAll(dir, 0o755)
		fmt.Fprintf(os.Stderr, "%s\n", f)
		var ref string
		for _, j := range judges {
			if !j.avail() {
				fr.Verdicts = append(fr.Verdicts, verdict{Judge: j.name, Skipped: true, TextChar: -1})
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			t0 := time.Now()
			v := j.run(ctx, abs, dir)
			v.Ms = time.Since(t0).Milliseconds()
			cancel()
			if v.Judge == "poppler" && v.Render != "" {
				ref = v.Render
			} else if v.Render != "" && ref != "" {
				if m, p, err := compareRenders(ref, v.Render); err == nil {
					v.MeanDiff, v.DiffPct = m, p
				} else {
					v.Err = "compare: " + err.Error()
				}
			}
			fmt.Fprintf(os.Stderr, "  %-8s ok=%v pages=%d text=%d warn=%d diff=%.1f/%.1f%% %dms %s\n",
				v.Judge, v.OK, v.Pages, v.TextChar, v.Warnings, v.MeanDiff, v.DiffPct, v.Ms, v.Err)
			fr.Verdicts = append(fr.Verdicts, v)
		}
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
	b.WriteString("Cell format: `pages · text ratio · Δrender` — pages the judge reports (– if it reports none), " +
		"its extracted-text length as a ratio of poppler's (– if it extracts none), and its page-1 render's " +
		"distance from poppler's (mean grey Δ on 0–255 / share of pixels off by more than 48). " +
		"⚠n = n warning lines on stderr; ❌ = the judge failed to process the file.\n\n")

	b.WriteString("| PDF | Bytes |")
	for _, n := range names {
		b.WriteString(" " + n + " |")
	}
	b.WriteString("\n|---|---|")
	for range names {
		b.WriteString("---|")
	}
	b.WriteString("\n")
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
		b.WriteString("\n")
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
		parts = append(parts, fmt.Sprintf("%.2f", float64(v.TextChar)/float64(popplerText)))
	default:
		parts = append(parts, strconv.Itoa(v.TextChar))
	}
	if v.Judge == "poppler" {
		parts = append(parts, "ref")
	} else if v.Render != "" {
		parts = append(parts, fmt.Sprintf("Δ%.1f/%.1f%%", v.MeanDiff, v.DiffPct))
	} else {
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
