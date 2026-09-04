// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command html2pdf renders a static HTML file to a vector PDF.
//
//	html2pdf -in report.html -out report.pdf
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-pdfkit/html2pdf"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr *os.File) int {
	fs := flag.NewFlagSet("html2pdf", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "input HTML file (required)")
	out := fs.String("out", "out.pdf", "output PDF path")
	marginMm := fs.Float64("margin", 20, "page margin, in millimetres, on all four sides")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fmt.Fprintln(stderr, "html2pdf: -in is required")
		return 2
	}

	src, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(stderr, "html2pdf: read: %v\n", err)
		return 1
	}

	doc, err := html2pdf.Export(string(src), html2pdf.Options{MarginMm: *marginMm})
	if err != nil {
		fmt.Fprintf(stderr, "html2pdf: %v\n", err)
		return 1
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(stderr, "html2pdf: create: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := doc.Write(f); err != nil {
		fmt.Fprintf(stderr, "html2pdf: write: %v\n", err)
		return 1
	}
	return 0
}
