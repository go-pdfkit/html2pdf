// Command breakcheck renders the fragmentation fixture (fixtures/breaks.html)
// with html2pdf and reports, marker by marker, the page each lands on
// against Chrome's answer key (fixtures/breaks.expected.tsv, recorded once
// with `chrome --headless --print-to-pdf`): a page break that CSS asked for
// either happened where a browser put it or it did not.
//
//	go run ./cmd/breakcheck -html2pdf /tmp/html2pdf-bin
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() { os.Exit(run()) }

func run() int {
	bin := flag.String("html2pdf", "html2pdf", "html2pdf CLI binary")
	fixture := flag.String("fixture", "fixtures/breaks.html", "the fixture")
	expected := flag.String("expected", "fixtures/breaks.expected.tsv", "Chrome's marker → page key")
	out := flag.String("out", "out/breaks.html2pdf.pdf", "where to write the PDF")
	flag.Parse()

	abs, _ := filepath.Abs(*fixture)
	if err := exec.Command(*bin, "-in", abs, "-out", *out).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "breakcheck: render: %v\n", err)
		return 1
	}
	pages := pageCount(*out)
	texts := make([]string, pages+1)
	for p := 1; p <= pages; p++ {
		b, _ := exec.Command("pdftotext", "-f", strconv.Itoa(p), "-l", strconv.Itoa(p), *out, "-").Output()
		texts[p] = string(b)
	}
	f, err := os.Open(*expected)
	if err != nil {
		fmt.Fprintf(os.Stderr, "breakcheck: %v\n", err)
		return 1
	}
	defer f.Close()
	agree, total := 0, 0
	sc := bufio.NewScanner(f)
	fmt.Printf("%-12s %6s %8s\n", "marker", "chrome", "html2pdf")
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		marker, want := parts[0], parts[1]
		got := ""
		for p := 1; p <= pages; p++ {
			if strings.Contains(texts[p], marker) {
				got += strconv.Itoa(p)
			}
		}
		// "6|7": either page is right — the constraint (orphans/widows) allows
		// both and only the font decides; Chrome's own answer is listed first.
		mark := "  "
		total++
		for _, alt := range strings.Split(want, "|") {
			if got == alt {
				agree++
				mark = "ok"
				break
			}
		}
		fmt.Printf("%-12s %6s %8s %s\n", marker, want, got, mark)
	}
	fmt.Printf("\npages: html2pdf %d; markers on the same page as Chrome: %d/%d\n", pages, agree, total)
	if agree != total {
		return 2
	}
	return 0
}

func pageCount(path string) int {
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
